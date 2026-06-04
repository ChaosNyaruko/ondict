package store

import (
	"context"
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
