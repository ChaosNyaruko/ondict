package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/ChaosNyaruko/ondict/dbutil"
)

// SQLiteHistory is the on-disk HistoryStore implementation.
type SQLiteHistory struct {
	db *sql.DB
}

// OpenSQLiteHistory opens the history SQLite DB at path and runs pending
// migrations. The parent directory is created automatically.
func OpenSQLiteHistory(path string) (*SQLiteHistory, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("open history %q: mkdir parent: %w", path, err)
		}
	}
	// WAL mode + busy_timeout + single connection — same rationale as
	// OpenSQLiteWordbank: prevents concurrent-write corruption and allows
	// the DB to survive a crash without a partially-applied journal.
	db, err := sql.Open("sqlite3", "file:"+path+"?_journal_mode=WAL&_busy_timeout=10000")
	if err != nil {
		return nil, fmt.Errorf("open history %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if err := dbutil.EnsureSchema(db, historyMigrations); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate history %q: %w", path, err)
	}
	return &SQLiteHistory{db: db}, nil
}

func (h *SQLiteHistory) Close() error {
	if h == nil || h.db == nil {
		return nil
	}
	return h.db.Close()
}

func (h *SQLiteHistory) Append(ctx context.Context, word string) error {
	word, err := normalizeWord(word)
	if err != nil {
		if errors.Is(err, ErrEmptyWord) {
			return nil
		}
		return err
	}
	// Stamp create_time / update_time explicitly in RFC3339-millisecond UTC
	// instead of letting them fall back to the legacy v1 column DEFAULT
	// (datetime('now','localtime') for first-insert, CURRENT_TIMESTAMP for
	// later UPDATEs). The legacy default produces "YYYY-MM-DD HH:MM:SS"
	// which time.Parse(time.RFC3339,…) cannot decode, breaking syncclient's
	// push cursor advancement and the wire-format invariant from ADR 0001.
	_, err = h.db.ExecContext(ctx,
		`INSERT INTO history (word, `+"`count`"+`, create_time, update_time, server_seen_at)
		 VALUES (?, 1, `+nowMs+`, `+nowMs+`, `+nowMs+`)
		 ON CONFLICT(word) DO UPDATE SET
		     `+"`count`"+`        = CASE WHEN deleted_at IS NULL THEN `+"`count`"+` + 1 ELSE 1 END,
		     update_time    = `+nowMs+`,
		     deleted_at     = NULL,
		     server_seen_at = `+nowMs,
		word,
	)
	return err
}

func (h *SQLiteHistory) List(ctx context.Context) ([]HistoryRow, error) {
	return h.queryHistory(ctx,
		`SELECT word, `+"`count`"+`, create_time, update_time, deleted_at, server_seen_at FROM history
		 WHERE deleted_at IS NULL
		 ORDER BY update_time DESC, word ASC`,
	)
}

func (h *SQLiteHistory) ListSince(ctx context.Context, t time.Time) ([]HistoryRow, error) {
	if t.IsZero() {
		return h.queryHistory(ctx,
			`SELECT word, `+"`count`"+`, create_time, update_time, deleted_at, server_seen_at FROM history
			 ORDER BY server_seen_at ASC, word ASC`,
		)
	}
	return h.queryHistory(ctx,
		`SELECT word, `+"`count`"+`, create_time, update_time, deleted_at, server_seen_at FROM history
		 WHERE server_seen_at > ?
		 ORDER BY server_seen_at ASC, word ASC`,
		t.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
	)
}

// ListChanged — see SQLiteWordbank.ListChanged for the rationale.
func (h *SQLiteHistory) ListChanged(ctx context.Context, since, cutoff time.Time) ([]HistoryRow, error) {
	const fmtMs = "2006-01-02T15:04:05.000Z07:00"
	cutoffStr := cutoff.UTC().Format(fmtMs)
	if since.IsZero() {
		return h.queryHistory(ctx,
			`SELECT word, `+"`count`"+`, create_time, update_time, deleted_at, server_seen_at FROM history
			 WHERE server_seen_at <= ?
			 ORDER BY server_seen_at ASC, word ASC`,
			cutoffStr,
		)
	}
	return h.queryHistory(ctx,
		`SELECT word, `+"`count`"+`, create_time, update_time, deleted_at, server_seen_at FROM history
		 WHERE server_seen_at > ? AND server_seen_at <= ?
		 ORDER BY server_seen_at ASC, word ASC`,
		since.UTC().Format(fmtMs),
		cutoffStr,
	)
}

func (h *SQLiteHistory) Review(ctx context.Context, days, count int) ([]HistoryRow, error) {
	// Compute the cutoff in Go and pass it as an RFC3339-millisecond UTC
	// string so it lives in the same string-format space as the stored
	// update_time column. SQLite's datetime('now','-N days') returns a
	// space-separated form ("YYYY-MM-DD HH:MM:SS") whose lexicographic
	// comparison against our stored RFC3339Z values is wrong (T = 0x54 is
	// greater than space = 0x20, so a row at 05:00 incorrectly compares
	// "greater than" a cutoff at 10:30 of the same day).
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02T15:04:05.000Z07:00")
	rows, err := h.db.QueryContext(ctx,
		`SELECT word, `+"`count`"+`, create_time, update_time, deleted_at FROM history
		   WHERE deleted_at IS NULL
		     AND update_time > ?
		     AND `+"`count`"+` >= ?
		   ORDER BY update_time DESC`,
		cutoff,
		count,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HistoryRow
	for rows.Next() {
		var (
			r       HistoryRow
			deleted sql.NullString
		)
		if err := rows.Scan(&r.Word, &r.Count, &r.CreateTime, &r.UpdateTime, &deleted); err != nil {
			return nil, fmt.Errorf("scan history review row: %w", err)
		}
		if deleted.Valid {
			r.DeletedAt = deleted.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (h *SQLiteHistory) Upsert(ctx context.Context, row HistoryRow) error {
	if row.Word == "" {
		return ErrEmptyWord
	}
	var deletedAt any
	if row.DeletedAt == "" {
		deletedAt = nil
	} else {
		deletedAt = row.DeletedAt
	}
	_, err := h.db.ExecContext(ctx,
		`INSERT INTO history (word, `+"`count`"+`, create_time, update_time, deleted_at, server_seen_at)
		 VALUES (?, ?, ?, ?, ?, `+nowMs+`)
		 ON CONFLICT(word) DO UPDATE SET
		     `+"`count`"+`        = excluded.`+"`count`"+`,
		     create_time    = excluded.create_time,
		     update_time    = excluded.update_time,
		     deleted_at     = excluded.deleted_at,
		     server_seen_at = `+nowMs,
		row.Word, row.Count, row.CreateTime, row.UpdateTime, deletedAt,
	)
	return err
}

func (h *SQLiteHistory) queryHistory(ctx context.Context, q string, args ...any) ([]HistoryRow, error) {
	rows, err := h.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HistoryRow
	for rows.Next() {
		var (
			r       HistoryRow
			deleted sql.NullString
			seen    sql.NullString
		)
		if err := rows.Scan(&r.Word, &r.Count, &r.CreateTime, &r.UpdateTime, &deleted, &seen); err != nil {
			return nil, fmt.Errorf("scan history row: %w", err)
		}
		if deleted.Valid {
			r.DeletedAt = deleted.String
		}
		if seen.Valid {
			r.SeenAt = seen.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// historyMigrations mirrors the migration slice in package history.
var historyMigrations = []dbutil.Migration{
	{
		Version: 1,
		Name:    "create_history_localtime_legacy",
		Apply: func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS history (
	word TEXT NOT NULL UNIQUE,
	` + "`count`" + ` INTEGER NOT NULL DEFAULT 0,
	create_time DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
	update_time DATETIME NOT NULL DEFAULT (datetime('now','localtime'))
);`)
			return err
		},
	},
	{
		Version: 2,
		Name:    "normalize_to_utc_and_add_tombstone",
		Apply: func(tx *sql.Tx) error {
			has, err := dbutil.ColumnExists(tx, "history", "deleted_at")
			if err != nil {
				return err
			}
			if !has {
				if _, err := tx.Exec(`ALTER TABLE history ADD COLUMN deleted_at DATETIME`); err != nil {
					return err
				}
			}
			if err := normalizeLegacyHistoryTimestamps(tx); err != nil {
				return fmt.Errorf("normalize legacy timestamps: %w", err)
			}
			if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS i_count ON history(` + "`count`" + `)`); err != nil {
				return err
			}
			if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS i_latest ON history(update_time)`); err != nil {
				return err
			}
			return nil
		},
	},
	{
		Version: 3,
		Name:    "add_server_seen_at",
		Apply: func(tx *sql.Tx) error {
			// See the wordbank v3 migration for the full rationale; in
			// short, server_seen_at is a sync-only filter column distinct
			// from the user-facing update_time. SQLite forbids non-const
			// DEFAULTs in ALTER, so we add nullable + backfill in two
			// statements.
			has, err := dbutil.ColumnExists(tx, "history", "server_seen_at")
			if err != nil {
				return err
			}
			if !has {
				if _, err := tx.Exec(
					`ALTER TABLE history ADD COLUMN server_seen_at DATETIME`,
				); err != nil {
					return err
				}
				if _, err := tx.Exec(
					`UPDATE history SET server_seen_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE server_seen_at IS NULL`,
				); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(
				`CREATE INDEX IF NOT EXISTS i_history_server_seen_at ON history(server_seen_at)`,
			); err != nil {
				return err
			}
			return nil
		},
	},
	{
		Version: 4,
		Name:    "canonicalize_defaults_to_rfc3339_ms",
		Apply: func(tx *sql.Tx) error {
			// Same motivation as wordbank v4: bring on-disk DEFAULTs in
			// line with schema.sql so manual sqlite3 inserts produce
			// rows in the canonical RFC3339-ms format.
			//
			// history is more important than wordbank here: pre-v4 the
			// `update_time`/`create_time` defaults were the v0/v1
			// LEGACY `datetime('now','localtime')` form — anyone who
			// hand-INSERTs without supplying timestamps would create
			// rows that look like the very localtime mess v2 was built
			// to clean up.
			return rebuildHistoryTableV4(tx)
		},
	},
}

// rebuildHistoryTableV4 swaps the history table for one whose column
// DEFAULTs match the canonical schema, AND normalises every existing
// row's create_time/update_time/deleted_at to the canonical RFC3339-ms
// UTC layout. See rebuildWordsTableV4 in sqlite_wordbank.go for the
// rationale (it applies symmetrically here).
//
// Pre-v4 history rows can carry several legacy shapes:
//   - v0/v1 raw `datetime('now','localtime')` output that the
//     ncruces driver auto-Z-suffixed (data already normalised to UTC
//     RFC3339Z by the v2 migration);
//   - v2-and-later rows written with the runtime nowMs constant
//     (already canonical);
//   - rows produced by manual sqlite3 inserts that fell through to
//     the old DEFAULT (CURRENT_TIMESTAMP, "YYYY-MM-DD HH:MM:SS" UTC).
//
// SQLite's strftime accepts all three shapes and re-emits them in
// ms-resolution UTC; we COALESCE through to the wall clock for any
// value strftime can't parse to keep NOT NULL columns satisfied.
func rebuildHistoryTableV4(tx *sql.Tx) error {
	const nowMsSQL = `strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`
	canonical := func(col string) string {
		return `COALESCE(strftime('%Y-%m-%dT%H:%M:%fZ', ` + col + `), ` + nowMsSQL + `)`
	}
	canonicalNullable := func(col string) string {
		return `CASE WHEN ` + col + ` IS NULL THEN NULL ELSE ` + canonical(col) + ` END`
	}

	if _, err := tx.Exec(`CREATE TABLE history_new (
	word           TEXT NOT NULL UNIQUE,
	` + "`count`" + ` INTEGER NOT NULL DEFAULT 0,
	create_time    DATETIME NOT NULL DEFAULT (` + nowMsSQL + `),
	update_time    DATETIME NOT NULL DEFAULT (` + nowMsSQL + `),
	deleted_at     DATETIME,
	server_seen_at DATETIME NOT NULL DEFAULT (` + nowMsSQL + `)
)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO history_new (word, ` + "`count`" + `, create_time, update_time, deleted_at, server_seen_at)
		SELECT word, ` + "`count`" + `,
		       ` + canonical("create_time") + `,
		       ` + canonical("update_time") + `,
		       ` + canonicalNullable("deleted_at") + `,
		       COALESCE(` + canonical("server_seen_at") + `, ` + nowMsSQL + `)
		FROM history`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE history`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE history_new RENAME TO history`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS i_count                   ON history(` + "`count`" + `)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS i_latest                  ON history(update_time)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS i_history_server_seen_at  ON history(server_seen_at)`); err != nil {
		return err
	}
	return nil
}

// normalizeLegacyHistoryTimestamps mirrors the helper in package history; see
// the comments there for the full backstory on driver auto-formatting and
// localtime-with-Z-suffix legacy values.
func normalizeLegacyHistoryTimestamps(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT word, create_time, update_time FROM history`)
	if err != nil {
		return err
	}
	type fix struct {
		word   string
		ct, ut string
	}
	var fixes []fix
	for rows.Next() {
		var word, ct, ut string
		if err := rows.Scan(&word, &ct, &ut); err != nil {
			rows.Close()
			return err
		}
		newCT, ctChanged := legacyLocaltimeToUTC(ct)
		newUT, utChanged := legacyLocaltimeToUTC(ut)
		if ctChanged || utChanged {
			fixes = append(fixes, fix{word: word, ct: newCT, ut: newUT})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	stmt, err := tx.Prepare(`UPDATE history SET create_time = ?, update_time = ? WHERE word = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, f := range fixes {
		if _, err := stmt.Exec(f.ct, f.ut, f.word); err != nil {
			return err
		}
	}
	return nil
}

func legacyLocaltimeToUTC(s string) (string, bool) {
	if t, err := time.Parse("2006-01-02T15:04:05Z", s); err == nil {
		shifted := time.Date(t.Year(), t.Month(), t.Day(),
			t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.Local)
		return shifted.UTC().Format(time.RFC3339), true
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t.UTC().Format(time.RFC3339), true
	}
	return s, false
}

// GCTombstones removes tombstone rows whose deleted_at is strictly older
// than `before`. Live rows are never touched.
func (h *SQLiteHistory) GCTombstones(ctx context.Context, before time.Time) (int, error) {
	res, err := h.db.ExecContext(ctx,
		`DELETE FROM history WHERE deleted_at IS NOT NULL AND deleted_at < ?`,
		before.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// silence unused-import lint when errors is not directly referenced after refactor.
var _ = errors.New
