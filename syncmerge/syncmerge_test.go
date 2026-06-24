package syncmerge

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ChaosNyaruko/ondict/store"
)

// ---------------------------------------------------------------------------
// Pure helpers — table-driven tests.
// ---------------------------------------------------------------------------

func TestMergeWordbankRow_TableDriven(t *testing.T) {
	cases := []struct {
		name     string
		existing *store.WordbankRow
		incoming store.WordbankRow
		want     store.WordbankRow
		wantAct  action
	}{
		{
			name:     "insert when not present",
			existing: nil,
			incoming: store.WordbankRow{Word: "alpha", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-01-01T00:00:00Z"},
			want:     store.WordbankRow{Word: "alpha", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-01-01T00:00:00Z"},
			wantAct:  actionInsert,
		},
		{
			name:     "incoming newer wins; create_time = MIN",
			existing: &store.WordbankRow{Word: "a", CreateTime: "2024-06-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
			incoming: store.WordbankRow{Word: "a", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z"},
			want:     store.WordbankRow{Word: "a", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z"},
			wantAct:  actionUpdate,
		},
		{
			name:     "existing newer wins; create_time still = MIN",
			existing: &store.WordbankRow{Word: "a", CreateTime: "2024-06-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z"},
			incoming: store.WordbankRow{Word: "a", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
			want:     store.WordbankRow{Word: "a", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z"},
			wantAct:  actionUpdate, // create_time changed
		},
		{
			name:     "exactly equal — noop",
			existing: &store.WordbankRow{Word: "a", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
			incoming: store.WordbankRow{Word: "a", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
			want:     store.WordbankRow{Word: "a", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
			wantAct:  actionNoop,
		},
		{
			name:     "incoming tombstone with newer update_time wins",
			existing: &store.WordbankRow{Word: "a", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
			incoming: store.WordbankRow{Word: "a", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z", DeletedAt: "2024-12-01T00:00:00Z"},
			want:     store.WordbankRow{Word: "a", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z", DeletedAt: "2024-12-01T00:00:00Z"},
			wantAct:  actionUpdate,
		},
		{
			name:     "incoming tombstone with older update_time loses (resurrection-by-readd preserved)",
			existing: &store.WordbankRow{Word: "a", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z"},
			incoming: store.WordbankRow{Word: "a", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z", DeletedAt: "2024-06-01T00:00:00Z"},
			want:     store.WordbankRow{Word: "a", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z"},
			wantAct:  actionNoop,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			present := tc.existing != nil
			var ex store.WordbankRow
			if present {
				ex = *tc.existing
			}
			got, act := mergeWordbankRow(ex, present, tc.incoming)
			require.Equal(t, tc.wantAct, act)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestMergeHistoryRow_TableDriven(t *testing.T) {
	cases := []struct {
		name     string
		existing *store.HistoryRow
		incoming store.HistoryRow
		want     store.HistoryRow
		wantAct  action
	}{
		{
			name:     "insert when not present",
			existing: nil,
			incoming: store.HistoryRow{Word: "doctor", Count: 3, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-01-01T00:00:00Z"},
			want:     store.HistoryRow{Word: "doctor", Count: 3, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-01-01T00:00:00Z"},
			wantAct:  actionInsert,
		},
		{
			name:     "count is MAX even when existing wins by update_time",
			existing: &store.HistoryRow{Word: "doctor", Count: 9, CreateTime: "2024-06-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z"},
			incoming: store.HistoryRow{Word: "doctor", Count: 12, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
			want:     store.HistoryRow{Word: "doctor", Count: 12, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z"},
			wantAct:  actionUpdate,
		},
		{
			name:     "count is MAX when incoming wins by update_time too",
			existing: &store.HistoryRow{Word: "doctor", Count: 7, CreateTime: "2024-06-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
			incoming: store.HistoryRow{Word: "doctor", Count: 5, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z"},
			want:     store.HistoryRow{Word: "doctor", Count: 7, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z"},
			wantAct:  actionUpdate,
		},
		{
			name:     "exact equality — noop",
			existing: &store.HistoryRow{Word: "doctor", Count: 4, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
			incoming: store.HistoryRow{Word: "doctor", Count: 4, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
			want:     store.HistoryRow{Word: "doctor", Count: 4, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
			wantAct:  actionNoop,
		},
		{
			name:     "tombstone propagation respects update_time",
			existing: &store.HistoryRow{Word: "doctor", Count: 4, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z"},
			incoming: store.HistoryRow{Word: "doctor", Count: 4, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z", DeletedAt: "2024-12-01T00:00:00Z"},
			want:     store.HistoryRow{Word: "doctor", Count: 4, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z", DeletedAt: "2024-12-01T00:00:00Z"},
			wantAct:  actionUpdate,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			present := tc.existing != nil
			var ex store.HistoryRow
			if present {
				ex = *tc.existing
			}
			got, act := mergeHistoryRow(ex, present, tc.incoming)
			require.Equal(t, tc.wantAct, act)
			require.Equal(t, tc.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// End-to-end tests against real SQLite stores.
// ---------------------------------------------------------------------------

func openTempWordbank(t *testing.T, name string) *store.SQLiteWordbank {
	t.Helper()
	wb, err := store.OpenSQLiteWordbank(filepath.Join(t.TempDir(), name))
	require.NoError(t, err)
	t.Cleanup(func() { _ = wb.Close() })
	return wb
}

func openTempHistory(t *testing.T, name string) *store.SQLiteHistory {
	t.Helper()
	h, err := store.OpenSQLiteHistory(filepath.Join(t.TempDir(), name))
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })
	return h
}

func TestMergeWordbank_EndToEnd(t *testing.T) {
	ctx := context.Background()
	dst := openTempWordbank(t, "dst.db")
	src := openTempWordbank(t, "src.db")

	require.NoError(t, dst.Upsert(ctx, store.WordbankRow{
		Word: "shared", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z",
	}))
	require.NoError(t, dst.Upsert(ctx, store.WordbankRow{
		Word: "dst-only", CreateTime: "2024-02-01T00:00:00Z", UpdateTime: "2024-07-01T00:00:00Z",
	}))

	require.NoError(t, src.Upsert(ctx, store.WordbankRow{
		Word: "shared", CreateTime: "2023-12-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z",
	}))
	require.NoError(t, src.Upsert(ctx, store.WordbankRow{
		Word: "src-only", CreateTime: "2024-03-01T00:00:00Z", UpdateTime: "2024-03-01T00:00:00Z",
	}))

	stats, err := MergeWordbank(ctx, dst, src)
	require.NoError(t, err)
	require.Equal(t, 1, stats.Inserted, "src-only must be inserted")
	require.Equal(t, 1, stats.Updated, "shared must be updated")
	require.Equal(t, 0, stats.Unchanged)

	rows, err := dst.ListSince(ctx, parseTimeStr(""))
	require.NoError(t, err)
	require.Len(t, rows, 3)
	byWord := map[string]store.WordbankRow{}
	for _, r := range rows {
		byWord[r.Word] = r
	}
	require.Equal(t, "2023-12-01T00:00:00Z", byWord["shared"].CreateTime, "shared.create_time = MIN")
	require.Equal(t, "2024-12-01T00:00:00Z", byWord["shared"].UpdateTime, "shared.update_time = MAX")
}

func TestMergeWordbank_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	dst := openTempWordbank(t, "dst.db")
	src := openTempWordbank(t, "src.db")

	require.NoError(t, src.Upsert(ctx, store.WordbankRow{
		Word: "alpha", CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-01-01T00:00:00Z",
	}))

	first, err := MergeWordbank(ctx, dst, src)
	require.NoError(t, err)
	require.Equal(t, 1, first.Inserted)

	second, err := MergeWordbank(ctx, dst, src)
	require.NoError(t, err)
	require.Equal(t, 0, second.Inserted)
	require.Equal(t, 0, second.Updated)
	require.Equal(t, 1, second.Unchanged, "second merge must be a pure no-op")
}

func TestMergeHistory_CountIsMAX_NotSUM(t *testing.T) {
	ctx := context.Background()
	dst := openTempHistory(t, "dst.db")
	src := openTempHistory(t, "src.db")

	require.NoError(t, dst.Upsert(ctx, store.HistoryRow{
		Word: "doctor", Count: 5, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z",
	}))
	require.NoError(t, src.Upsert(ctx, store.HistoryRow{
		Word: "doctor", Count: 5, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z",
	}))

	_, err := MergeHistory(ctx, dst, src)
	require.NoError(t, err)
	_, err = MergeHistory(ctx, dst, src)
	require.NoError(t, err) // running twice must still produce count = 5.

	rows, err := dst.List(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 5, rows[0].Count, "count must be MAX, not SUM (ADR D3)")
}

func TestMergeHistory_TombstonePropagates(t *testing.T) {
	ctx := context.Background()
	dst := openTempHistory(t, "dst.db")
	src := openTempHistory(t, "src.db")

	require.NoError(t, dst.Upsert(ctx, store.HistoryRow{
		Word: "doctor", Count: 5, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-06-01T00:00:00Z",
	}))
	require.NoError(t, src.Upsert(ctx, store.HistoryRow{
		Word: "doctor", Count: 5, CreateTime: "2024-01-01T00:00:00Z", UpdateTime: "2024-12-01T00:00:00Z",
		DeletedAt: "2024-12-01T00:00:00Z",
	}))

	stats, err := MergeHistory(ctx, dst, src)
	require.NoError(t, err)
	require.Equal(t, 1, stats.TombstonesPropagated)

	live, err := dst.List(ctx)
	require.NoError(t, err)
	require.Empty(t, live, "tombstoned row must not be in live List()")
}

func TestStats_Add(t *testing.T) {
	a := Stats{Inserted: 1, Updated: 2, Unchanged: 3, TombstonesPropagated: 4}
	b := Stats{Inserted: 10, Updated: 20, Unchanged: 30, TombstonesPropagated: 40}
	got := a.Add(b)
	require.Equal(t, Stats{Inserted: 11, Updated: 22, Unchanged: 33, TombstonesPropagated: 44}, got)
}
