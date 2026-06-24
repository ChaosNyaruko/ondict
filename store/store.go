// Package store defines the storage interfaces used by wordbank and history,
// plus their SQLite-backed implementations. The split is the foundation for
// the merge tool (offline DB-to-DB folding) and the cloud-sync server/client.
//
// See docs/adr/0001-wordbank-history-sync.md for the design rationale.
//
// # Conventions
//
//   - All timestamps in [WordbankRow] / [HistoryRow] are RFC3339 UTC strings
//     with millisecond precision (e.g. "2026-06-04T07:09:51.234Z").
//   - A row is "live" when DeletedAt == ""; otherwise it is a tombstone.
//   - The user-facing methods (Add / Append / Remove) stamp the current
//     instant via SQLite's `strftime('%Y-%m-%dT%H:%M:%fZ', 'now')` and are
//     the path the HTTP handlers and CLI use. (Earlier versions used
//     `CURRENT_TIMESTAMP`, which has only second resolution and a
//     space-separator that mis-sorts against the RFC3339 form used
//     everywhere else; see ADR D11.)
//   - The sync-facing methods (Upsert, ListSince, ListChanged) take
//     explicit row state so the merge engine can preserve the originating
//     device's timestamps; SeenAt on the row is set by the local DB at
//     write time and is the field ListSince/ListChanged filter on.
package store

import (
	"context"
	"io"
	"time"
)

// WordbankRow is the wire/merge representation of a single wordbank entry.
// All timestamps are RFC3339 UTC. DeletedAt is empty for live rows.
//
// SeenAt is the local-write wall-clock time (server_seen_at) of this row in
// the originating database. It is exposed only on rows returned by
// ListSince so the sync client can advance its push cursor in the same
// time domain that ListSince filters on (see ADR 0001 / D11). It is NOT
// part of the wire push payload and is ignored on Upsert.
type WordbankRow struct {
	Word       string
	CreateTime string
	UpdateTime string
	DeletedAt  string
	SeenAt     string
}

// HistoryRow is the wire/merge representation of a single history entry.
// SeenAt has the same semantics as WordbankRow.SeenAt.
type HistoryRow struct {
	Word       string
	Count      int
	CreateTime string
	UpdateTime string
	DeletedAt  string
	SeenAt     string
}

// WordbankStore is the abstraction over a wordbank backing store.
// Implementations must be safe for concurrent use across goroutines.
type WordbankStore interface {
	io.Closer

	// Add records a user-initiated save of word. Equivalent to the legacy
	// package-level wordbank.Add. Implementations stamp UpdateTime with the
	// current UTC instant and clear any tombstone.
	Add(ctx context.Context, word string) error

	// Remove soft-deletes word: the row stays present with a non-empty
	// DeletedAt so the tombstone can propagate via sync.
	Remove(ctx context.Context, word string) error

	// Contains reports whether word is currently present and not tombstoned.
	Contains(ctx context.Context, word string) (bool, error)

	// List returns all live rows (tombstones excluded), ordered by
	// UpdateTime DESC, Word ASC.
	List(ctx context.Context) ([]WordbankRow, error)

	// ListSince returns every row (live AND tombstoned) whose
	// server_seen_at (local-write wall-clock time) is strictly greater
	// than t. The zero time returns the full snapshot. Used by the sync
	// layer to compute deltas. See ADR 0001 for the server_seen_at vs
	// update_time distinction.
	ListSince(ctx context.Context, t time.Time) ([]WordbankRow, error)

	// ListChanged returns every row (live AND tombstoned) whose
	// server_seen_at falls in the half-open interval (since, cutoff].
	// Capturing the cutoff before scanning lets the server avoid losing
	// rows that are written concurrently with a pull: any write whose
	// server_seen_at lands after cutoff is excluded here and surfaces on
	// the next pull. See ADR / code-review P1.
	ListChanged(ctx context.Context, since, cutoff time.Time) ([]WordbankRow, error)

	// Upsert applies a row with caller-provided timestamps. This is the
	// merge / sync path; conflict resolution lives in the syncmerge package
	// and is not the store's responsibility — Upsert simply writes what it
	// is told. Implementations must respect the input create_time /
	// update_time / deleted_at verbatim (no rewriting them to "now"), but
	// server_seen_at is always stamped to the local "now" so newly-arrived
	// rows surface in the next pull.
	Upsert(ctx context.Context, row WordbankRow) error

	// GCTombstones permanently removes tombstoned rows whose deleted_at is
	// strictly less than `before`. Returns the number of rows removed.
	// Callers should choose `before` conservatively (e.g. now-90d) so all
	// other devices have already had a chance to observe the deletion.
	GCTombstones(ctx context.Context, before time.Time) (int, error)
}

// HistoryStore is the abstraction over a history backing store.
type HistoryStore interface {
	io.Closer

	// Append records a user query of word: increments Count, bumps
	// UpdateTime, clears any tombstone, and resets Count to 1 if the row
	// was tombstoned (resurrection).
	Append(ctx context.Context, word string) error

	// List returns every live row, ordered UpdateTime DESC.
	List(ctx context.Context) ([]HistoryRow, error)

	// ListSince returns every row (live AND tombstoned) whose
	// server_seen_at is strictly greater than t.
	ListSince(ctx context.Context, t time.Time) ([]HistoryRow, error)

	// ListChanged returns every row (live AND tombstoned) whose
	// server_seen_at falls in the half-open interval (since, cutoff].
	// See WordbankStore.ListChanged for the rationale.
	ListChanged(ctx context.Context, since, cutoff time.Time) ([]HistoryRow, error)

	// Review returns rows updated within the last `days` days with
	// Count >= count, newest first. Equivalent to the legacy
	// (*History).Review but row-shaped instead of pre-formatted text.
	Review(ctx context.Context, days, count int) ([]HistoryRow, error)

	// Upsert applies a row with caller-provided fields verbatim. Same
	// contract as WordbankStore.Upsert.
	Upsert(ctx context.Context, row HistoryRow) error

	// GCTombstones permanently removes tombstoned rows whose deleted_at is
	// strictly less than `before`. Returns the number of rows removed.
	GCTombstones(ctx context.Context, before time.Time) (int, error)
}
