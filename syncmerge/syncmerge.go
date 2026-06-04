// Package syncmerge folds wordbank and history rows from one or more source
// stores into a destination store.
//
// The engine is pure: every decision flows from the input rows alone, so the
// same code drives the offline `ondict merge` CLI (Phase 4 / ADR D8) and the
// cloud-sync server (Phase 5).
//
// # Conflict resolution
//
// All rules are derived from ADR 0001:
//
//   - D1  Sync at the row level. Each row is keyed by `word`.
//   - D2  Last-writer-wins by UpdateTime; CreateTime = MIN(create_a, create_b);
//     the tombstone (DeletedAt) wins iff it carries the larger UpdateTime.
//   - D3  History `count` is merged with MAX (idempotent). SUM was rejected
//     because the same source DB merged twice would double-count.
//
// All timestamps are RFC3339 UTC (post schema v2; see ADR / Schema versions).
// Empty / unparseable timestamps are treated as the zero time and lose every
// comparison.
package syncmerge

import (
	"context"
	"time"

	"github.com/ChaosNyaruko/ondict/store"
)

// Stats summarises the outcome of a merge run.
type Stats struct {
	// Inserted counts rows that did not exist in the destination.
	Inserted int
	// Updated counts rows where the destination already had a row but the
	// source brought a strictly newer UpdateTime (or a different field).
	Updated int
	// Unchanged counts rows whose source UpdateTime was <= destination's
	// (no-ops). Helpful for "is this a real merge or just a redundant
	// re-import?" diagnostics.
	Unchanged int
	// TombstonesPropagated counts updates that turned a previously-live
	// destination row into a tombstone (or moved a tombstone forward).
	TombstonesPropagated int
}

// Add combines two Stats. Useful when merging multiple sources sequentially.
func (s Stats) Add(o Stats) Stats {
	return Stats{
		Inserted:             s.Inserted + o.Inserted,
		Updated:              s.Updated + o.Updated,
		Unchanged:            s.Unchanged + o.Unchanged,
		TombstonesPropagated: s.TombstonesPropagated + o.TombstonesPropagated,
	}
}

// MergeWordbank folds every row from src into dst applying ADR-D2 conflict
// resolution. The function is idempotent: re-running it after a previous
// merge produces only Unchanged increments.
func MergeWordbank(ctx context.Context, dst, src store.WordbankStore) (Stats, error) {
	srcRows, err := src.ListSince(ctx, time.Time{})
	if err != nil {
		return Stats{}, err
	}
	dstRows, err := dst.ListSince(ctx, time.Time{})
	if err != nil {
		return Stats{}, err
	}
	dstByWord := make(map[string]store.WordbankRow, len(dstRows))
	for _, r := range dstRows {
		dstByWord[r.Word] = r
	}

	var stats Stats
	for _, in := range srcRows {
		existing, present := dstByWord[in.Word]
		merged, action := mergeWordbankRow(existing, present, in)
		switch action {
		case actionInsert:
			if err := dst.Upsert(ctx, merged); err != nil {
				return stats, err
			}
			stats.Inserted++
			if merged.DeletedAt != "" {
				stats.TombstonesPropagated++
			}
		case actionUpdate:
			if err := dst.Upsert(ctx, merged); err != nil {
				return stats, err
			}
			stats.Updated++
			if existing.DeletedAt == "" && merged.DeletedAt != "" {
				stats.TombstonesPropagated++
			}
		case actionNoop:
			stats.Unchanged++
		}
	}
	return stats, nil
}

// MergeHistory folds every row from src into dst applying ADR-D2 + D3.
func MergeHistory(ctx context.Context, dst, src store.HistoryStore) (Stats, error) {
	srcRows, err := src.ListSince(ctx, time.Time{})
	if err != nil {
		return Stats{}, err
	}
	dstRows, err := dst.ListSince(ctx, time.Time{})
	if err != nil {
		return Stats{}, err
	}
	dstByWord := make(map[string]store.HistoryRow, len(dstRows))
	for _, r := range dstRows {
		dstByWord[r.Word] = r
	}

	var stats Stats
	for _, in := range srcRows {
		existing, present := dstByWord[in.Word]
		merged, action := mergeHistoryRow(existing, present, in)
		switch action {
		case actionInsert:
			if err := dst.Upsert(ctx, merged); err != nil {
				return stats, err
			}
			stats.Inserted++
			if merged.DeletedAt != "" {
				stats.TombstonesPropagated++
			}
		case actionUpdate:
			if err := dst.Upsert(ctx, merged); err != nil {
				return stats, err
			}
			stats.Updated++
			if existing.DeletedAt == "" && merged.DeletedAt != "" {
				stats.TombstonesPropagated++
			}
		case actionNoop:
			stats.Unchanged++
		}
	}
	return stats, nil
}

// ---------------------------------------------------------------------------
// Pure helpers — exported via the unit tests.
// ---------------------------------------------------------------------------

type action int

const (
	actionNoop action = iota
	actionInsert
	actionUpdate
)

func mergeWordbankRow(existing store.WordbankRow, present bool, in store.WordbankRow) (store.WordbankRow, action) {
	if !present {
		return in, actionInsert
	}
	merged := pickWordbankWinner(existing, in)
	merged.CreateTime = minTimeStr(existing.CreateTime, in.CreateTime)
	if wordbankRowEqual(existing, merged) {
		return merged, actionNoop
	}
	return merged, actionUpdate
}

func mergeHistoryRow(existing store.HistoryRow, present bool, in store.HistoryRow) (store.HistoryRow, action) {
	if !present {
		return in, actionInsert
	}
	merged := pickHistoryWinner(existing, in)
	merged.CreateTime = minTimeStr(existing.CreateTime, in.CreateTime)
	// D3: count = MAX. We compute this independently of LWW because Count
	// can come from the "loser" side (the device that recorded more queries
	// before the other syncrhonised).
	if existing.Count > merged.Count {
		merged.Count = existing.Count
	}
	if in.Count > merged.Count {
		merged.Count = in.Count
	}
	if historyRowEqual(existing, merged) {
		return merged, actionNoop
	}
	return merged, actionUpdate
}

// pickWordbankWinner returns the row whose UpdateTime is greater. Ties go to
// `in` because in practice the caller is folding `in` *into* `existing`, and
// "incoming wins ties" makes idempotent re-merges trivial: re-merging the
// same source again will never re-write a row whose dst snapshot is already
// equal to in.
func pickWordbankWinner(existing, in store.WordbankRow) store.WordbankRow {
	if compareTimeStr(existing.UpdateTime, in.UpdateTime) > 0 {
		return existing
	}
	return in
}

func pickHistoryWinner(existing, in store.HistoryRow) store.HistoryRow {
	if compareTimeStr(existing.UpdateTime, in.UpdateTime) > 0 {
		return existing
	}
	return in
}

func wordbankRowEqual(a, b store.WordbankRow) bool {
	return a.Word == b.Word &&
		a.CreateTime == b.CreateTime &&
		a.UpdateTime == b.UpdateTime &&
		a.DeletedAt == b.DeletedAt
}

func historyRowEqual(a, b store.HistoryRow) bool {
	return a.Word == b.Word &&
		a.Count == b.Count &&
		a.CreateTime == b.CreateTime &&
		a.UpdateTime == b.UpdateTime &&
		a.DeletedAt == b.DeletedAt
}

// compareTimeStr returns -1, 0, or +1 for a < b, a == b, a > b on RFC3339
// timestamps. Unparseable strings sort as the zero time (and therefore lose
// every comparison against parseable strings).
func compareTimeStr(a, b string) int {
	ta := parseTimeStr(a)
	tb := parseTimeStr(b)
	switch {
	case ta.Before(tb):
		return -1
	case ta.After(tb):
		return +1
	default:
		return 0
	}
}

// minTimeStr returns the lexically/temporally smaller of two RFC3339 strings,
// preserving the original string format. Empty strings are ignored: an empty
// CreateTime never displaces a populated one.
func minTimeStr(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if compareTimeStr(a, b) <= 0 {
		return a
	}
	return b
}

func parseTimeStr(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	// Fall back to the legacy space-separated localtime format. Treat as
	// time.Local for the comparison; the merge engine is forgiving about
	// pre-migration data shapes.
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local); err == nil {
		return t
	}
	return time.Time{}
}
