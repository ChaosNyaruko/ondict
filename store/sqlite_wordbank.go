package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/ChaosNyaruko/ondict/dbutil"
)

// ErrEmptyWord is returned by mutator methods when the input word is empty
// after trimming.
var ErrEmptyWord = errors.New("word is empty")

// SQLiteWordbank is the on-disk WordbankStore implementation. The file path
// is fixed at construction; concurrent use is safe (database/sql provides
// its own connection pool).
type SQLiteWordbank struct {
	db *sql.DB
}

// OpenSQLiteWordbank opens (or creates) the wordbank SQLite DB at path,
// running any pending migrations. Returns an error if the file cannot be
// opened or the schema cannot be brought up to date. The parent directory
// is created automatically.
func OpenSQLiteWordbank(path string) (*SQLiteWordbank, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("open wordbank %q: mkdir parent: %w", path, err)
		}
	}
	// Use WAL journal mode so that readers never block writers and a crash
	// cannot leave the main DB file partially written. busy_timeout gives
	// concurrent writers up to 10 s to acquire the write lock before
	// returning SQLITE_BUSY. MaxOpenConns(1) keeps the connection pool at
	// a single connection, which is all SQLite needs for serialised writes
	// and is the safest setting when WAL mode is not yet confirmed active.
	db, err := sql.Open("sqlite3", "file:"+path+"?_journal_mode=WAL&_busy_timeout=10000")
	if err != nil {
		return nil, fmt.Errorf("open wordbank %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if err := dbutil.EnsureSchema(db, wordbankMigrations); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate wordbank %q: %w", path, err)
	}
	return &SQLiteWordbank{db: db}, nil
}

// Close releases the underlying *sql.DB.
func (w *SQLiteWordbank) Close() error {
	if w == nil || w.db == nil {
		return nil
	}
	return w.db.Close()
}

// nowMs is the SQL fragment for "current UTC instant with millisecond
// resolution as RFC3339". CURRENT_TIMESTAMP only has second granularity
// which causes flakiness in sync delta tests when multiple writes land in
// the same second.
const nowMs = "strftime('%Y-%m-%dT%H:%M:%fZ', 'now')"

func (w *SQLiteWordbank) Add(ctx context.Context, word string) error {
	word, err := normalizeWord(word)
	if err != nil {
		return err
	}
	// Stamp every timestamp explicitly in RFC3339-millisecond UTC so the
	// row format matches what syncclient/server expect on the wire. Falling
	// back to the column DEFAULT would give us CURRENT_TIMESTAMP (second
	// resolution, space-separated) which then mixes with the millisecond
	// strings written by the sync path and breaks downstream time.Parse.
	_, err = w.db.ExecContext(ctx, `
INSERT INTO words (word, create_time, update_time, server_seen_at)
VALUES (?, `+nowMs+`, `+nowMs+`, `+nowMs+`)
ON CONFLICT(word) DO UPDATE SET update_time = `+nowMs+`, deleted_at = NULL, server_seen_at = `+nowMs+`;
`, word)
	return err
}

func (w *SQLiteWordbank) Remove(ctx context.Context, word string) error {
	word, err := normalizeWord(word)
	if err != nil {
		return err
	}
	_, err = w.db.ExecContext(ctx, `UPDATE words
		SET deleted_at = `+nowMs+`, update_time = `+nowMs+`, server_seen_at = `+nowMs+`
		WHERE word = ? AND deleted_at IS NULL`, word)
	return err
}

func (w *SQLiteWordbank) Contains(ctx context.Context, word string) (bool, error) {
	word, err := normalizeWord(word)
	if err != nil {
		if errors.Is(err, ErrEmptyWord) {
			return false, nil
		}
		return false, err
	}
	var found int
	err = w.db.QueryRowContext(ctx,
		`SELECT 1 FROM words WHERE word = ? AND deleted_at IS NULL LIMIT 1`,
		word,
	).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (w *SQLiteWordbank) List(ctx context.Context) ([]WordbankRow, error) {
	return w.queryWordbank(ctx,
		`SELECT word, create_time, update_time, deleted_at, server_seen_at FROM words
		 WHERE deleted_at IS NULL
		 ORDER BY update_time DESC, word ASC`,
	)
}

func (w *SQLiteWordbank) ListSince(ctx context.Context, t time.Time) ([]WordbankRow, error) {
	if t.IsZero() {
		return w.queryWordbank(ctx,
			`SELECT word, create_time, update_time, deleted_at, server_seen_at FROM words
			 ORDER BY server_seen_at ASC, word ASC`,
		)
	}
	// Filter by server_seen_at (the row's local-write wall-clock time)
	// rather than update_time. update_time can be backdated by an Upsert
	// from another device and would otherwise hide newly-arrived rows.
	// Millisecond format matches the strftime('%f') values stored on disk
	// so lexicographic SQL comparisons are correct.
	return w.queryWordbank(ctx,
		`SELECT word, create_time, update_time, deleted_at, server_seen_at FROM words
		 WHERE server_seen_at > ?
		 ORDER BY server_seen_at ASC, word ASC`,
		t.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
	)
}

// ListChanged returns rows whose server_seen_at is in (since, cutoff].
// The closed upper bound prevents the pull-time race documented in
// ADR / code-review P1.
func (w *SQLiteWordbank) ListChanged(ctx context.Context, since, cutoff time.Time) ([]WordbankRow, error) {
	const fmtMs = "2006-01-02T15:04:05.000Z07:00"
	cutoffStr := cutoff.UTC().Format(fmtMs)
	if since.IsZero() {
		return w.queryWordbank(ctx,
			`SELECT word, create_time, update_time, deleted_at, server_seen_at FROM words
			 WHERE server_seen_at <= ?
			 ORDER BY server_seen_at ASC, word ASC`,
			cutoffStr,
		)
	}
	return w.queryWordbank(ctx,
		`SELECT word, create_time, update_time, deleted_at, server_seen_at FROM words
		 WHERE server_seen_at > ? AND server_seen_at <= ?
		 ORDER BY server_seen_at ASC, word ASC`,
		since.UTC().Format(fmtMs),
		cutoffStr,
	)
}

// Upsert writes the row verbatim. If the incoming row has an empty
// DeletedAt the on-disk deleted_at column is set to NULL (live row);
// otherwise the tombstone timestamp is preserved.
//
// server_seen_at is always stamped with CURRENT_TIMESTAMP — it tracks when
// THIS process learned about the row, not the row's own update_time.
func (w *SQLiteWordbank) Upsert(ctx context.Context, row WordbankRow) error {
	if strings.TrimSpace(row.Word) == "" {
		return ErrEmptyWord
	}
	var deletedAt any
	if row.DeletedAt == "" {
		deletedAt = nil
	} else {
		deletedAt = row.DeletedAt
	}
	_, err := w.db.ExecContext(ctx, `
INSERT INTO words (word, create_time, update_time, deleted_at, server_seen_at)
VALUES (?, ?, ?, ?, `+nowMs+`)
ON CONFLICT(word) DO UPDATE SET
	create_time    = excluded.create_time,
	update_time    = excluded.update_time,
	deleted_at     = excluded.deleted_at,
	server_seen_at = `+nowMs+``,
		row.Word, row.CreateTime, row.UpdateTime, deletedAt,
	)
	return err
}

func (w *SQLiteWordbank) queryWordbank(ctx context.Context, q string, args ...any) ([]WordbankRow, error) {
	rows, err := w.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WordbankRow
	for rows.Next() {
		var (
			r       WordbankRow
			deleted sql.NullString
			seen    sql.NullString
		)
		if err := rows.Scan(&r.Word, &r.CreateTime, &r.UpdateTime, &deleted, &seen); err != nil {
			return nil, fmt.Errorf("scan wordbank row: %w", err)
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

func normalizeWord(word string) (string, error) {
	word = strings.TrimSpace(word)
	if word == "" {
		return "", ErrEmptyWord
	}
	return word, nil
}

// GCTombstones removes tombstone rows whose deleted_at is strictly older
// than `before`. Live rows are never touched.
func (w *SQLiteWordbank) GCTombstones(ctx context.Context, before time.Time) (int, error) {
	res, err := w.db.ExecContext(ctx,
		`DELETE FROM words WHERE deleted_at IS NOT NULL AND deleted_at < ?`,
		before.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// wordbankMigrations mirrors the migration list in package wordbank.
// Both sites use the same schema; keeping the slice here lets store be
// importable without depending on package wordbank.
var wordbankMigrations = []dbutil.Migration{
	{
		Version: 1,
		Name:    "create_words",
		Apply: func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS words (
	word TEXT NOT NULL PRIMARY KEY COLLATE NOCASE,
	create_time DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
	update_time DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP)
);`)
			return err
		},
	},
	{
		Version: 2,
		Name:    "add_deleted_at_tombstone",
		Apply: func(tx *sql.Tx) error {
			has, err := dbutil.ColumnExists(tx, "words", "deleted_at")
			if err != nil {
				return err
			}
			if !has {
				if _, err := tx.Exec(`ALTER TABLE words ADD COLUMN deleted_at DATETIME`); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(
				`CREATE INDEX IF NOT EXISTS i_words_update_time ON words(update_time)`,
			); err != nil {
				return err
			}
			return nil
		},
	},
	{
		Version: 3,
		Name:    "add_server_seen_at",
		Apply: func(tx *sql.Tx) error {
			// server_seen_at is the local wall-clock time at which this
			// row was last written, regardless of the user-content
			// update_time. The sync protocol filters by this column so
			// pushed rows with backdated update_times still propagate to
			// other devices on the next pull (see ADR 0001 wire-protocol
			// follow-ups).
			//
			// SQLite forbids non-constant DEFAULTs in ALTER ADD COLUMN, so
			// we add the column without a default, backfill with the
			// current wall-clock time (so existing rows look "just-seen"
			// on first sync), then expect future writes to stamp it
			// explicitly.
			has, err := dbutil.ColumnExists(tx, "words", "server_seen_at")
			if err != nil {
				return err
			}
			if !has {
				if _, err := tx.Exec(
					`ALTER TABLE words ADD COLUMN server_seen_at DATETIME`,
				); err != nil {
					return err
				}
				if _, err := tx.Exec(
					`UPDATE words SET server_seen_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE server_seen_at IS NULL`,
				); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(
				`CREATE INDEX IF NOT EXISTS i_words_server_seen_at ON words(server_seen_at)`,
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
			// Earlier migrations left the column DEFAULTs in
			// inconsistent states: v1 used CURRENT_TIMESTAMP, v3
			// ALTERs added columns without any default. The runtime
			// always writes timestamps via strftime('%f','now') so it
			// works regardless, but `schema.sql` documents the canonical
			// shape and someone manually `INSERT`-ing via sqlite3 CLI
			// would otherwise get rows whose timestamps don't match the
			// sync delta filter format.
			//
			// SQLite cannot change column DEFAULTs in place, so we
			// rebuild the table: create a new table with the canonical
			// shape, copy rows over, drop the old table, rename the new
			// one. All inside the migration's transaction.
			return rebuildWordsTableV4(tx)
		},
	},
}

// rebuildWordsTableV4 swaps the words table for one whose column DEFAULTs
// match the canonical schema documented in schema.sql, AND normalises
// every existing row's create_time/update_time/deleted_at to the
// canonical RFC3339-ms UTC layout.
//
// The data-side normalisation matters because pre-v4 rows may carry
// SQLite CURRENT_TIMESTAMP output ("YYYY-MM-DD HH:MM:SS", UTC) — which
// downstream parsers (notably syncmerge.parseTimeStr's fallback) treat
// as time.Local and therefore mis-resolve LWW comparisons by the local
// timezone offset. We re-emit every legacy row in the canonical layout
// during the rebuild to remove that footgun for good.
func rebuildWordsTableV4(tx *sql.Tx) error {
	const nowMsSQL = `strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`

	// canonical(x) returns the canonical RFC3339-ms UTC form of x. SQLite's
	// strftime accepts ISO8601 with 'Z' (kept as-is, ms-padded if needed)
	// and the legacy "YYYY-MM-DD HH:MM:SS" form (which CURRENT_TIMESTAMP
	// produces in UTC; strftime treats this as UTC too — no timezone
	// shift). NULL or unparseable strftime results fall through to the
	// current wall-clock so we never end up with NULL in a NOT NULL column.
	canonical := func(col string) string {
		return `COALESCE(strftime('%Y-%m-%dT%H:%M:%fZ', ` + col + `), ` + nowMsSQL + `)`
	}
	// canonicalNullable: same as above but preserves NULL inputs (used for
	// deleted_at, where NULL means "not a tombstone").
	canonicalNullable := func(col string) string {
		return `CASE WHEN ` + col + ` IS NULL THEN NULL ELSE ` + canonical(col) + ` END`
	}

	if _, err := tx.Exec(`CREATE TABLE words_new (
	word           TEXT NOT NULL PRIMARY KEY COLLATE NOCASE,
	create_time    DATETIME NOT NULL DEFAULT (` + nowMsSQL + `),
	update_time    DATETIME NOT NULL DEFAULT (` + nowMsSQL + `),
	deleted_at     DATETIME,
	server_seen_at DATETIME NOT NULL DEFAULT (` + nowMsSQL + `)
)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO words_new (word, create_time, update_time, deleted_at, server_seen_at)
		SELECT word,
		       ` + canonical("create_time") + `,
		       ` + canonical("update_time") + `,
		       ` + canonicalNullable("deleted_at") + `,
		       COALESCE(` + canonical("server_seen_at") + `, ` + nowMsSQL + `)
		FROM words`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE words`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE words_new RENAME TO words`); err != nil {
		return err
	}
	// Indexes on the old table got dropped along with it; recreate them
	// against the new table.
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS i_words_update_time    ON words(update_time)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS i_words_server_seen_at ON words(server_seen_at)`); err != nil {
		return err
	}
	return nil
}
