package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func openWB(t *testing.T) *SQLiteWordbank {
	t.Helper()
	wb, err := OpenSQLiteWordbank(filepath.Join(t.TempDir(), "wordbank.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = wb.Close() })
	return wb
}

func TestSQLiteWordbank_AddListContainsRemove(t *testing.T) {
	ctx := context.Background()
	wb := openWB(t)

	require.NoError(t, wb.Add(ctx, " apple "))
	require.NoError(t, wb.Add(ctx, "banana"))

	got, err := wb.Contains(ctx, "apple")
	require.NoError(t, err)
	require.True(t, got)

	rows, err := wb.List(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	require.NoError(t, wb.Remove(ctx, "apple"))
	got, err = wb.Contains(ctx, "apple")
	require.NoError(t, err)
	require.False(t, got)
}

func TestSQLiteWordbank_RemoveSoftDeletes(t *testing.T) {
	ctx := context.Background()
	wb := openWB(t)

	require.NoError(t, wb.Add(ctx, "apple"))
	require.NoError(t, wb.Remove(ctx, "apple"))

	live, err := wb.List(ctx)
	require.NoError(t, err)
	require.Empty(t, live)

	all, err := wb.ListSince(ctx, time.Time{})
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.NotEmpty(t, all[0].DeletedAt, "tombstone deleted_at should be populated")
}

func TestSQLiteWordbank_UpsertPreservesIncomingTimestamps(t *testing.T) {
	ctx := context.Background()
	wb := openWB(t)

	row := WordbankRow{
		Word:       "imported",
		CreateTime: "2024-01-02T03:04:05Z",
		UpdateTime: "2024-06-07T08:09:10Z",
	}
	require.NoError(t, wb.Upsert(ctx, row))

	rows, err := wb.List(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, row.CreateTime, rows[0].CreateTime)
	require.Equal(t, row.UpdateTime, rows[0].UpdateTime)
}

func TestSQLiteWordbank_UpsertTombstone(t *testing.T) {
	ctx := context.Background()
	wb := openWB(t)

	require.NoError(t, wb.Upsert(ctx, WordbankRow{
		Word:       "ghost",
		CreateTime: "2024-01-01T00:00:00Z",
		UpdateTime: "2024-01-02T00:00:00Z",
		DeletedAt:  "2024-01-02T00:00:00Z",
	}))

	got, err := wb.Contains(ctx, "ghost")
	require.NoError(t, err)
	require.False(t, got, "tombstoned upsert must not surface as live")

	all, err := wb.ListSince(ctx, time.Time{})
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, "2024-01-02T00:00:00Z", all[0].DeletedAt)
}

func TestSQLiteWordbank_ListSinceFilter(t *testing.T) {
	ctx := context.Background()
	wb := openWB(t)

	// ListSince filters by server_seen_at (local-write wall-clock time),
	// not the user-content update_time. We capture a cutoff in the middle
	// of two writes to verify only the second row is returned.
	require.NoError(t, wb.Upsert(ctx, WordbankRow{
		Word: "old", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-01-01T00:00:00Z",
	}))
	cutoff := time.Now().UTC()
	// SQLite's CURRENT_TIMESTAMP has 1-second granularity; sleep so the
	// second row's server_seen_at is strictly greater than `cutoff`.
	time.Sleep(1100 * time.Millisecond)
	require.NoError(t, wb.Upsert(ctx, WordbankRow{
		Word: "new", CreateTime: "2025-01-01T00:00:00Z", UpdateTime: "2025-01-01T00:00:00Z",
	}))

	rows, err := wb.ListSince(ctx, cutoff)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "new", rows[0].Word)
}

func openHist(t *testing.T) *SQLiteHistory {
	t.Helper()
	h, err := OpenSQLiteHistory(filepath.Join(t.TempDir(), "history.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })
	return h
}

func TestSQLiteHistory_AppendIncrementsCount(t *testing.T) {
	ctx := context.Background()
	h := openHist(t)

	require.NoError(t, h.Append(ctx, "doctor"))
	require.NoError(t, h.Append(ctx, "doctor"))
	require.NoError(t, h.Append(ctx, "doctor"))

	rows, err := h.List(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 3, rows[0].Count)
}

func TestSQLiteHistory_UpsertRoundtrip(t *testing.T) {
	ctx := context.Background()
	h := openHist(t)

	row := HistoryRow{
		Word:       "imported",
		Count:      7,
		CreateTime: "2024-01-02T03:04:05Z",
		UpdateTime: "2024-06-07T08:09:10Z",
	}
	require.NoError(t, h.Upsert(ctx, row))

	got, err := h.List(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, row.Count, got[0].Count)
	require.Equal(t, row.CreateTime, got[0].CreateTime)
	require.Equal(t, row.UpdateTime, got[0].UpdateTime)
}

func TestSQLiteHistory_TombstoneResurrection(t *testing.T) {
	ctx := context.Background()
	h := openHist(t)

	require.NoError(t, h.Append(ctx, "ephemeral"))
	require.NoError(t, h.Append(ctx, "ephemeral"))

	// Manually tombstone via Upsert to mimic a sync deletion.
	require.NoError(t, h.Upsert(ctx, HistoryRow{
		Word:       "ephemeral",
		Count:      2,
		CreateTime: "2024-01-01T00:00:00Z",
		UpdateTime: "2024-01-02T00:00:00Z",
		DeletedAt:  "2024-01-02T00:00:00Z",
	}))

	rows, err := h.List(ctx)
	require.NoError(t, err)
	require.Empty(t, rows, "tombstoned row must not appear in List()")

	// Re-append resurrects with count=1 (per ADR D3 / Append contract).
	require.NoError(t, h.Append(ctx, "ephemeral"))
	rows, err = h.List(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 1, rows[0].Count, "resurrected row must reset count to 1")
}

func TestSQLiteHistory_Review(t *testing.T) {
	ctx := context.Background()
	h := openHist(t)
	require.NoError(t, h.Append(ctx, "alpha"))
	require.NoError(t, h.Append(ctx, "alpha"))
	require.NoError(t, h.Append(ctx, "beta"))

	rows, err := h.Review(ctx, 1, 2)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "alpha", rows[0].Word)
}

func TestSQLiteWordbank_GCTombstones(t *testing.T) {
	ctx := context.Background()
	wb := openWB(t)

	// Old tombstone (should be GC'd) and a recent tombstone (should stay).
	require.NoError(t, wb.Upsert(ctx, WordbankRow{
		Word:       "old-tombstone",
		CreateTime: "2020-01-01T00:00:00Z",
		UpdateTime: "2020-01-01T00:00:00Z",
		DeletedAt:  "2020-01-01T00:00:00Z",
	}))
	require.NoError(t, wb.Upsert(ctx, WordbankRow{
		Word:       "live",
		CreateTime: "2025-01-01T00:00:00Z",
		UpdateTime: "2025-01-01T00:00:00Z",
	}))

	cutoff, err := time.Parse(time.RFC3339, "2024-01-01T00:00:00Z")
	require.NoError(t, err)
	removed, err := wb.GCTombstones(ctx, cutoff)
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	all, err := wb.ListSince(ctx, time.Time{})
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, "live", all[0].Word)
}

func TestSQLiteHistory_GCTombstones(t *testing.T) {
	ctx := context.Background()
	h := openHist(t)

	require.NoError(t, h.Upsert(ctx, HistoryRow{
		Word: "old", Count: 1, CreateTime: "2020-01-01T00:00:00Z", UpdateTime: "2020-01-01T00:00:00Z",
		DeletedAt: "2020-01-01T00:00:00Z",
	}))
	require.NoError(t, h.Upsert(ctx, HistoryRow{
		Word: "live", Count: 1, CreateTime: "2025-01-01T00:00:00Z", UpdateTime: "2025-01-01T00:00:00Z",
	}))

	cutoff, err := time.Parse(time.RFC3339, "2024-01-01T00:00:00Z")
	require.NoError(t, err)
	removed, err := h.GCTombstones(ctx, cutoff)
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	all, err := h.ListSince(ctx, time.Time{})
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, "live", all[0].Word)
}

// Regression for code-review P1b: ListChanged must be a half-open
// interval (since, cutoff] so the server can pre-capture cutoff and any
// row written after cutoff surfaces only on the next pull.
func TestSQLiteWordbank_ListChangedHalfOpenInterval(t *testing.T) {
	ctx := context.Background()
	wb := openWB(t)

	// row written at T0
	require.NoError(t, wb.Add(ctx, "first"))
	time.Sleep(2 * time.Millisecond)
	cutoff := time.Now().UTC()
	time.Sleep(2 * time.Millisecond)
	// row written strictly AFTER cutoff
	require.NoError(t, wb.Add(ctx, "after-cutoff"))

	rows, err := wb.ListChanged(ctx, time.Time{}, cutoff)
	require.NoError(t, err)
	for _, r := range rows {
		require.NotEqual(t, "after-cutoff", r.Word, "rows whose server_seen_at > cutoff must be excluded")
	}
	require.Len(t, rows, 1)
	require.Equal(t, "first", rows[0].Word)
}

// Regression for code-review P2: Add must populate create_time / update_time
// in RFC3339-millisecond UTC, not leave them at the column default. Without
// this, time.Parse(time.RFC3339, row.UpdateTime) fails downstream and the
// syncclient push cursor cannot advance.
func TestSQLiteWordbank_AddProducesParseableRFC3339Timestamps(t *testing.T) {
	ctx := context.Background()
	wb := openWB(t)

	require.NoError(t, wb.Add(ctx, "apple"))

	rows, err := wb.List(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	_, err = time.Parse(time.RFC3339, rows[0].CreateTime)
	require.NoError(t, err, "create_time %q is not RFC3339 parseable", rows[0].CreateTime)
	_, err = time.Parse(time.RFC3339, rows[0].UpdateTime)
	require.NoError(t, err, "update_time %q is not RFC3339 parseable", rows[0].UpdateTime)
}

func TestSQLiteHistory_AppendProducesParseableRFC3339Timestamps(t *testing.T) {
	ctx := context.Background()
	h := openHist(t)

	require.NoError(t, h.Append(ctx, "apple"))

	rows, err := h.List(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	_, err = time.Parse(time.RFC3339, rows[0].CreateTime)
	require.NoError(t, err, "create_time %q is not RFC3339 parseable", rows[0].CreateTime)
	_, err = time.Parse(time.RFC3339, rows[0].UpdateTime)
	require.NoError(t, err, "update_time %q is not RFC3339 parseable", rows[0].UpdateTime)
}

// Regression for code-review P2: Review compared SQLite's
// datetime('now',-Nd) (space-separated) against stored RFC3339Z values
// (T-separated). Lexicographic ordering of `T` (0x54) vs space (0x20)
// caused rows earlier in the day to be falsely included on the same date
// as the cutoff. Cover that exact case.
func TestSQLiteHistory_ReviewBoundaryComparisonIsCorrect(t *testing.T) {
	ctx := context.Background()
	h := openHist(t)

	// Seed a row whose update_time digits sort as "T05" — characters that
	// would lexically beat a space-separated cutoff "10:30" if we weren't
	// careful. We drive Review with days=0 so the cutoff is "now"; an
	// older row must be excluded.
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02T15:04:05.000Z07:00")
	require.NoError(t, h.Upsert(ctx, HistoryRow{
		Word:       "yesterday",
		Count:      9,
		CreateTime: yesterday,
		UpdateTime: yesterday,
	}))
	require.NoError(t, h.Append(ctx, "today"))

	rows, err := h.Review(ctx, 0, 1) // last 0 days, count >= 1 → only "today"
	require.NoError(t, err)
	for _, r := range rows {
		require.NotEqual(t, "yesterday", r.Word, "yesterday must NOT appear in last-0-days review (P2 boundary fix)")
	}
}

// Regression for code-review (round 3) P3b: schema.sql claims canonical
// DEFAULTs are strftime('%Y-%m-%dT%H:%M:%fZ', 'now'). The migration
// products must match — otherwise a manual `INSERT` from sqlite3 CLI
// (which falls through to the column DEFAULT) would create rows in a
// timestamp format the sync delta filter doesn't recognise.
//
// We assert by reading sqlite_master and looking for the canonical
// strftime fragment in the table DDL.
func TestSQLiteWordbank_PostMigrationSchemaMatchesCanonical(t *testing.T) {
	wb := openWB(t)
	var sql string
	err := wb.db.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type='table' AND name='words'`,
	).Scan(&sql)
	require.NoError(t, err)
	require.Contains(t, sql, "strftime('%Y-%m-%dT%H:%M:%fZ', 'now')",
		"words.create_time/update_time/server_seen_at DEFAULTs must be RFC3339-ms (got: %s)", sql)
	require.NotContains(t, sql, "CURRENT_TIMESTAMP",
		"words DEFAULTs must NOT be CURRENT_TIMESTAMP after v4 (got: %s)", sql)
}

func TestSQLiteHistory_PostMigrationSchemaMatchesCanonical(t *testing.T) {
	h := openHist(t)
	var sql string
	err := h.db.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type='table' AND name='history'`,
	).Scan(&sql)
	require.NoError(t, err)
	require.Contains(t, sql, "strftime('%Y-%m-%dT%H:%M:%fZ', 'now')",
		"history timestamp DEFAULTs must be RFC3339-ms (got: %s)", sql)
	require.NotContains(t, sql, "datetime('now','localtime')",
		"history DEFAULTs must NOT be the legacy localtime form after v4 (got: %s)", sql)
	require.NotContains(t, sql, "CURRENT_TIMESTAMP",
		"history DEFAULTs must NOT be CURRENT_TIMESTAMP after v4 (got: %s)", sql)
}

// Migration v4 must preserve every existing row (data migration safety).
func TestSQLiteWordbank_V4PreservesData(t *testing.T) {
	ctx := context.Background()
	wb := openWB(t)

	require.NoError(t, wb.Add(ctx, "alpha"))
	require.NoError(t, wb.Add(ctx, "beta"))
	require.NoError(t, wb.Remove(ctx, "alpha"))

	rows, err := wb.ListSince(ctx, time.Time{})
	require.NoError(t, err)
	require.Len(t, rows, 2, "v4 rebuild must preserve all rows incl. tombstones")

	live, err := wb.List(ctx)
	require.NoError(t, err)
	require.Len(t, live, 1)
	require.Equal(t, "beta", live[0].Word)
}

// Regression for code-review (round 4) P2/P3: the v4 rebuild must
// normalise existing rows' create_time / update_time / deleted_at into
// the canonical RFC3339-ms UTC layout, not just change the column
// DEFAULTs. We seed a hand-built v3-shape DB with the legacy
// CURRENT_TIMESTAMP form ("YYYY-MM-DD HH:MM:SS", UTC), set
// schema_version=3, then open via OpenSQLiteWordbank to drive the
// v3→v4 migration, and assert every timestamp on every row is now in
// the canonical form on disk.
//
// We probe the on-disk shape via CAST AS TEXT to bypass the driver's
// type-affinity coercion (which auto-Z-suffixes DATETIME column reads
// for second-precision values, masking the legacy storage form). The
// pre-fix verbatim INSERT…SELECT in v4 leaves "YYYY-MM-DD HH:MM:SS"
// on disk; the fix uses strftime to re-emit the canonical layout.
func TestSQLiteWordbank_V4NormalisesLegacyTimestampValues(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wordbank.db")

	// Build a v3-shape DB by hand. v3's words table has columns:
	//   word PK, create_time DEFAULT CURRENT_TIMESTAMP,
	//   update_time DEFAULT CURRENT_TIMESTAMP, deleted_at, server_seen_at
	// (added by v3's ALTER, no default).
	{
		db, err := sql.Open("sqlite3", "file:"+dbPath)
		require.NoError(t, err)
		_, err = db.Exec(`CREATE TABLE words (
			word TEXT NOT NULL PRIMARY KEY COLLATE NOCASE,
			create_time DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
			update_time DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP)
		)`)
		require.NoError(t, err)
		_, err = db.Exec(`ALTER TABLE words ADD COLUMN deleted_at DATETIME`)
		require.NoError(t, err)
		_, err = db.Exec(`ALTER TABLE words ADD COLUMN server_seen_at DATETIME`)
		require.NoError(t, err)
		// Seed three rows with the legacy "YYYY-MM-DD HH:MM:SS" form
		// (what CURRENT_TIMESTAMP produces). One live, one tombstoned.
		_, err = db.Exec(`INSERT INTO words VALUES
			('alpha', '2024-01-01 12:00:00', '2024-06-01 12:00:00', NULL,                  '2024-06-01 12:00:00'),
			('beta',  '2024-02-01 09:30:00', '2024-07-01 09:30:00', NULL,                  '2024-07-01 09:30:00'),
			('gamma', '2024-03-01 06:15:00', '2024-08-01 06:15:00', '2024-08-01 06:15:00', '2024-08-01 06:15:00')`)
		require.NoError(t, err)
		// Confirm the legacy form is actually on disk before migration.
		var raw string
		require.NoError(t, db.QueryRow(`SELECT CAST(update_time AS TEXT) FROM words WHERE word='alpha'`).Scan(&raw))
		require.Equal(t, "2024-06-01 12:00:00", raw, "test setup must seed the legacy form")
		// Pretend we are at schema_version=3 so EnsureSchema runs only v4.
		_, err = db.Exec(`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO meta VALUES ('schema_version', '3')`)
		require.NoError(t, err)
		require.NoError(t, db.Close())
	}

	// Open via the public API — this must drive v4 and normalise.
	wb, err := OpenSQLiteWordbank(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = wb.Close() })

	// Probe the on-disk values directly via CAST AS TEXT to defeat the
	// driver's type-affinity coercion of DATETIME columns. The canonical
	// form contains a literal 'T' separator and ms fragment; the legacy
	// form has a space separator and no Z.
	rows, err := wb.db.Query(`SELECT word,
		CAST(create_time    AS TEXT),
		CAST(update_time    AS TEXT),
		CAST(deleted_at     AS TEXT),
		CAST(server_seen_at AS TEXT)
		FROM words ORDER BY word`)
	require.NoError(t, err)
	defer rows.Close()

	got := map[string]struct {
		create, update, deleted, seen string
	}{}
	for rows.Next() {
		var word, ct, ut string
		var dt, ss sql.NullString
		require.NoError(t, rows.Scan(&word, &ct, &ut, &dt, &ss))
		got[word] = struct {
			create, update, deleted, seen string
		}{ct, ut, dt.String, ss.String}
	}
	require.Len(t, got, 3, "all seeded rows must survive the rebuild")

	for word, ts := range got {
		// Each timestamp must be in the canonical RFC3339-ms layout
		// (literal 'T' + '.' ms separator + 'Z'). The legacy form
		// (space + no Z) must not survive.
		for label, v := range map[string]string{
			"create_time": ts.create, "update_time": ts.update, "server_seen_at": ts.seen,
		} {
			require.Containsf(t, v, "T", "row %q %s on-disk %q must be canonical (T-separated)", word, label, v)
			require.Containsf(t, v, ".", "row %q %s on-disk %q must be canonical (ms fragment)", word, label, v)
			require.Containsf(t, v, "Z", "row %q %s on-disk %q must be canonical (Z suffix)", word, label, v)
			require.NotContainsf(t, v, " ", "row %q %s on-disk %q must NOT contain the legacy space separator", word, label, v)
		}
	}

	// gamma's deleted_at must also be normalised (NULL preserved for
	// the others).
	require.Equal(t, "", got["alpha"].deleted, "alpha live row must keep deleted_at NULL")
	require.Equal(t, "", got["beta"].deleted, "beta live row must keep deleted_at NULL")
	require.Contains(t, got["gamma"].deleted, "T", "gamma tombstone must be canonical")
	require.Contains(t, got["gamma"].deleted, ".", "gamma tombstone must be canonical")

	// Digit-preservation check: CURRENT_TIMESTAMP was already UTC, so
	// "06:01 12:00:00" must come out as "06-01T12:00:00.000Z" — NOT
	// shifted by the local timezone. This is the key invariant for
	// LWW correctness in the merge engine (ADR D2/D11).
	require.Equal(t, "2024-06-01T12:00:00.000Z", got["alpha"].update,
		"v4 must NOT shift legacy CURRENT_TIMESTAMP digits by local TZ")
	require.Equal(t, "2024-08-01T06:15:00.000Z", got["gamma"].deleted)
}

// Regression for code-review (round 4): same invariant as the wordbank
// test above, but for history. Pre-v4, history rows could be in either
// the v0/v1 `datetime('now','localtime')` form (already normalised by
// the v2 migration to RFC3339Z) or the legacy CURRENT_TIMESTAMP form
// (from manual inserts that fell through to a column DEFAULT). v4 must
// canonicalise both.
func TestSQLiteHistory_V4NormalisesLegacyTimestampValues(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "history.db")

	{
		db, err := sql.Open("sqlite3", "file:"+dbPath)
		require.NoError(t, err)
		// Build a v3-shape history table.
		_, err = db.Exec(`CREATE TABLE history (
			word TEXT NOT NULL UNIQUE,
			` + "`count`" + ` INTEGER NOT NULL DEFAULT 0,
			create_time DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
			update_time DATETIME NOT NULL DEFAULT (datetime('now','localtime'))
		)`)
		require.NoError(t, err)
		_, err = db.Exec(`ALTER TABLE history ADD COLUMN deleted_at DATETIME`)
		require.NoError(t, err)
		_, err = db.Exec(`ALTER TABLE history ADD COLUMN server_seen_at DATETIME`)
		require.NoError(t, err)
		// Mix two legacy shapes: one row with the
		// CURRENT_TIMESTAMP-style space form, another with the
		// driver-auto-Z-suffixed form that v2 produced.
		_, err = db.Exec(`INSERT INTO history VALUES
			('legacy_space', 4, '2024-01-01 12:00:00',  '2024-06-01 12:00:00',  NULL,                   '2024-06-01 12:00:00'),
			('legacy_z',     7, '2024-02-01T09:30:00Z', '2024-07-01T09:30:00Z', '2024-07-01T09:30:00Z', '2024-07-01T09:30:00Z')`)
		require.NoError(t, err)
		_, err = db.Exec(`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO meta VALUES ('schema_version', '3')`)
		require.NoError(t, err)
		require.NoError(t, db.Close())
	}

	h, err := OpenSQLiteHistory(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })

	rows, err := h.db.Query(`SELECT word,
		CAST(create_time    AS TEXT),
		CAST(update_time    AS TEXT),
		CAST(deleted_at     AS TEXT),
		CAST(server_seen_at AS TEXT)
		FROM history ORDER BY word`)
	require.NoError(t, err)
	defer rows.Close()

	got := map[string]struct {
		create, update, deleted, seen string
	}{}
	for rows.Next() {
		var word, ct, ut string
		var dt, ss sql.NullString
		require.NoError(t, rows.Scan(&word, &ct, &ut, &dt, &ss))
		got[word] = struct {
			create, update, deleted, seen string
		}{ct, ut, dt.String, ss.String}
	}
	require.Len(t, got, 2)

	for word, ts := range got {
		for label, v := range map[string]string{
			"create_time": ts.create, "update_time": ts.update, "server_seen_at": ts.seen,
		} {
			require.Containsf(t, v, "T", "row %q %s on-disk %q must be canonical (T-separated)", word, label, v)
			require.Containsf(t, v, ".", "row %q %s on-disk %q must be canonical (ms fragment)", word, label, v)
			require.Containsf(t, v, "Z", "row %q %s on-disk %q must be canonical (Z suffix)", word, label, v)
			require.NotContainsf(t, v, " ", "row %q %s on-disk %q must NOT contain the legacy space separator", word, label, v)
		}
	}

	// Digits preserved as UTC for both legacy shapes.
	require.Equal(t, "2024-06-01T12:00:00.000Z", got["legacy_space"].update,
		"legacy CURRENT_TIMESTAMP digits must NOT be TZ-shifted")
	require.Equal(t, "2024-07-01T09:30:00.000Z", got["legacy_z"].update,
		"legacy RFC3339Z digits must be ms-padded but otherwise preserved")
	require.Equal(t, "2024-07-01T09:30:00.000Z", got["legacy_z"].deleted,
		"legacy_z tombstone must be canonicalised")
	require.Equal(t, "", got["legacy_space"].deleted,
		"live row's NULL deleted_at must be preserved as NULL, not stamped to 'now'")
}
