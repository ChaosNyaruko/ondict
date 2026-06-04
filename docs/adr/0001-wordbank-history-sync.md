# ADR 0001 — Wordbank & History Merge / Cloud-Sync

- Status: **Accepted** — 2026-06-04
- Deciders: ondict maintainers
- Supersedes: —
- Superseded by: —

## Context

Ondict has two independent SQLite stores that hold per-user state:

- `wordbank.db` — table `words(word PK COLLATE NOCASE, create_time, update_time)`, timestamps in **UTC**.
- `history.db` — table `history(word UNIQUE, count, create_time, update_time)`, timestamps in **localtime** (inconsistent with wordbank).

These DBs survive on each device only. Two motivating use cases:

1. **Offline merge.** Given several `wordbank.db` / `history.db` files from different machines, fold them into one without manual review.
2. **Cloud sync.** Mobile app data (Android, in-process Go server at `127.0.0.1:1345`) is currently lost on reinstall / device change. We want to push/pull state to a long-lived self-hosted server so a user can move freely between devices.

Both use cases share the same primitive: **idempotent, conflict-aware merge of row-keyed records**.

The architecture splits the problem into:

```
Consumers (CLI merge tool, HTTP handlers, sync client)
        │
        ▼
Store interfaces (WordbankStore, HistoryStore)        — pkg `store/`
        │
        ▼
Backends (sqlite-local, remote-HTTP)
        │
        ▼
Merge engine (pure functions over the interfaces)     — pkg `syncmerge/`
```

## Decisions (summary)

| ID  | Topic                       | Chosen                                                 |
|-----|-----------------------------|--------------------------------------------------------|
| D1  | Sync granularity            | Row-level delta sync (not file replication, not CAS)   |
| D2  | Conflict resolution         | Last-writer-wins by `update_time`; `MIN(create_time)`  |
| D3  | History `count` merge       | `MAX` (idempotent)                                     |
| D4  | Auth                        | Single-user HTTP Basic Auth                            |
| D5  | Transport                   | HTTP REST + JSON on the existing Gin server            |
| D6  | Delete semantics            | Soft delete + tombstones                               |
| D7  | Wordbank refactor scope     | Interface + singleton shim (zero churn at call sites)  |
| D8  | Merge tool location         | `ondict merge` subcommand of the main binary           |
| D9  | Mobile history              | Re-enable on Android so it can be synced               |
| D10 | Hosting / TLS               | Self-hosted; TLS terminated by external reverse proxy  |
| D11 | Sync delta filter column    | `server_seen_at` (local-write wall-clock, ms-precision) |

## Per-decision detail

### D1 — Sync granularity: row-level delta sync

- **Problem.** How do we move state between devices without one device's writes clobbering another's?
- **Options considered.**
  - (a) Whole-file replication: phone uploads `wordbank.db`, server overwrites. **Rejected** — last-uploader-wins at file level destroys concurrent edits from other devices.
  - (b) Row-level delta sync: exchange individual rows, merge with upsert keyed on `word`, resolve via `update_time`. **Chosen.**
  - (c) Content-addressable storage (Git/IPFS-style hash trees). **Rejected** — fancy machinery for a single-table, ~few-thousand-row dataset; overkill.
  - (d) CRDT library (e.g. automerge). **Rejected for v1** — unnecessary complexity given a single user across devices, low write rate, and a clear authoritative server.
- **Rationale.** Both schemas already carry `update_time`, the dataset is tiny, and an upsert per row is trivial to implement and test.
- **Limitations.** Last-writer-wins loses information when two devices edit the same word offline. Acceptable for v1 — the lost edit is just a metadata refresh; the word itself stays.
- **Revisit when.** Multi-writer real-time editing matters, or we want guaranteed lossless merges → consider CRDT.

### D2 — Conflict resolution: LWW by `update_time` + `MIN(create_time)`

- **Problem.** When two records collide on the same `word`, which fields win?
- **Chosen rule.**
  - Identity row wins by `MAX(update_time)` (LWW).
  - `create_time` always becomes `MIN(create_time_a, create_time_b)` so the earliest discovery is preserved.
  - Tombstone wins over a non-tombstone iff its `update_time` is greater (delete-then-readd works correctly).
- **Limitations.** Requires clocks that aren't wildly skewed. We accept that risk for personal devices.
- **Revisit when.** We add multi-user or cross-trust-boundary sync.

### D3 — History `count` merge: MAX

- **Problem.** When the same `word` exists in two history DBs with counts `a` and `b`, what should the merged count be?
- **Options.**
  - (a) `MAX(a, b)`. Idempotent — merging the same DB twice yields no change. **Chosen.**
  - (b) `SUM(a, b)`. Truthful for *disjoint* histories, but double-counts when the same DB is merged repeatedly or pulled from a server that already has the data.
  - (c) Append-only event log: one row per query. SUM becomes safe via per-event dedup. Bigger schema rewrite; bigger DB on disk.
- **Rationale.** Idempotence is the cheapest guarantee; users care about *which* words they look up more than the exact magnitude.
- **Limitations.** Frequency totals across devices will under-report.
- **Revisit when.** Users ask for accurate cross-device frequency / time-series analytics → migrate to event log (option c).

### D4 — Auth: single-user HTTP Basic Auth

- **Problem.** Sync server must reject unauthorized writes; sync client must authenticate.
- **Options.**
  - (a) **HTTP Basic Auth, single user.** **Chosen.** Matches the existing cookie-login simplicity and the explicit hint in `todo.md:94`.
  - (b) Per-user accounts + bcrypt + user table. Needed only if multiple humans share the server.
  - (c) OAuth / OIDC (Google, GitHub). Right for a hosted SaaS; unnecessary for self-hosted single-user.
  - (d) mTLS or pre-shared key headers. Stronger; more operational pain.
- **Limitations.** Single set of credentials; rotation is manual; brute-force resistance depends on rate-limiting at the reverse proxy.
- **Revisit when.** Sharing with another user; deploying as a service; threat model tightens.

### D5 — Transport: HTTP REST + JSON

- **Problem.** Wire protocol between sync client and server.
- **Options.**
  - (a) **HTTP REST + JSON on the existing Gin server.** **Chosen.** Trivial to test with `curl`; reuses existing routing/middleware; humans can read it.
  - (b) gRPC. Typed, smaller wire, streaming — but adds proto toolchain and bigger Android binary.
  - (c) WebSocket push. Server-initiated invalidation. Useful only when we want near-realtime updates.
- **Wire endpoints (v1).**
  ```
  POST /sync/v1/wordbank/pull   { since } → { items[], server_now }
  POST /sync/v1/wordbank/push   { items[] } → { applied, conflicts? }
  POST /sync/v1/history/pull    { since } → { items[], server_now }
  POST /sync/v1/history/push    { items[] } → { applied }
  GET  /sync/v1/state                       → { server_now, schema_version }
  ```
- **Revisit when.** Real-time multi-device updates, or payload size becomes a bottleneck.

### D6 — Delete: soft delete + tombstones

- **Problem.** A delete on device A must propagate to device B.
- **Options.**
  - (a) **Soft delete with `deleted_at` column on the row** (or a `tombstones` table). **Chosen.** Tombstone propagates via the same row sync path; LWW handles delete-then-readd correctly.
  - (b) Hard delete — deletes don't sync. Simpler, but device B keeps zombies.
- **GC.** Tombstones that are older than N days (default 90) and globally observed by all known clients can be vacuumed. v1 ships with a manual GC entry point; automation later.
- **Revisit when.** Tombstone bloat becomes visible.

### D7 — Wordbank refactor: interface + singleton shim

- **Problem.** `wordbank.Add/Remove/List/Contains` are package-level functions called from many sites (HTTP handlers, tests, mobile path, CLI).
- **Options.**
  - (a) **Introduce `WordbankStore` interface; keep package-level functions as thin shims over a process-singleton store.** **Chosen.** Zero churn at call sites; sync simply attaches a different impl behind the singleton.
  - (b) Migrate every call site to take an injected interface.
- **Rationale.** Minimum-disruption v1; we can lift to (b) later if injection is needed at every call site.
- **Revisit when.** Tests need to inject fakes at every call site, or we want multiple wordbanks per process.

### D8 — Merge tool location: `ondict merge` subcommand

- **Options.**
  - (a) **Subcommand of the existing `ondict` binary.** **Chosen.** Reuses store impls, single distribution artefact, single config story.
  - (b) Standalone `cmd/ondict-merge/` binary.
- **CLI shape (v1).**
  ```
  ondict merge wordbank --dst out.db src1.db src2.db [--dry-run]
  ondict merge history  --dst out.db src1.db src2.db [--dry-run]
  ```
- **Revisit when.** The main binary becomes too large or merge gains heavy deps the server shouldn't carry.

### D9 — Mobile history: re-enable on Android

- **Problem.** `mobile/mobile.go:63` currently passes `History: nil` ("no history recording on mobile"). With sync, mobile should record so its history shows up on desktop.
- **Chosen.** Re-enable the SQLite history writer on Android once the schema migration (Phase 1) lands.
- **Limitations.** Slight write amplification on every query. Acceptable.
- **Revisit when.** Privacy concerns or storage pressure surface.

### D10 — Hosting / TLS: self-host; TLS at reverse proxy

- **Options.**
  - (a) **Self-hosted (home box / VPS) behind Caddy/nginx for TLS.** **Chosen.** Matches the "no telemetry, your data on your server" philosophy of ondict; lets us focus on protocol logic.
  - (b) Built-in TLS via `autocert` / Let's Encrypt. Lower deployment friction, more code in the binary.
  - (c) Hosted multi-tenant SaaS. Out of scope.
- **Revisit when.** We want zero-deps deployment for non-technical users → switch to (b).

### D11 — Sync delta filter column: `server_seen_at` (millisecond-precision)

- **Problem.** A delta-sync cursor needs to answer "give me every row that landed in this database after timestamp X". The naive choice is to filter by the existing `update_time` column. That breaks because `update_time` is **content-time** (when the user edited the word), often heavily backdated relative to the moment the row arrived in the database via a push.
- **Concrete failure mode.** Server wall-clock time `T1`. Device A pulls (cursor stored = `T1`), then pushes alpha with `update_time="2024-06-01"`. At `T2 > T1`, device B pulls and pushes beta with `update_time="2024-07-01"`. At `T3 > T2`, device A pulls again with cursor=`T1`. If the server filters `WHERE update_time > T1`, the comparison reduces to `"2024-07-01" > T1` (where `T1` is some wall-clock instant such as `2026-06-04T07:00:00Z`) — false. Beta is never delivered to device A.
- **Options.**
  - (a) Filter by `update_time`. **Rejected** for the reason above.
  - (b) Add a separate `server_seen_at` column, stamped to the local wall-clock instant on every write, and filter by it. **Chosen.** Cleanly separates content-time from observation-time; merge semantics (D2) keep using `update_time` and are unaffected.
  - (c) Coerce `update_time = MAX(now, incoming.update_time)` on server ingestion. **Rejected.** Pollutes the merge winner: a backdated push from device A would suddenly look "newer" than an earlier-pushed row from device B. Breaks ADR D2's invariant.
  - (d) Use a per-write monotonic version counter instead of a timestamp. **Rejected for v1.** Cleaner in theory but requires server-side coordination and a bigger protocol surface; revisit if clock skew across devices ever becomes a real problem.
- **Precision.** SQLite's `CURRENT_TIMESTAMP` has 1-second granularity. Two writes within the same second collapse to the same string, and lexicographic comparison of `2026-06-04T07:09:51Z` against `2026-06-04T07:09:51.500Z` is wrong (`.` < `Z` in ASCII, so the second-resolution Z-suffix string is "greater" than the millisecond-resolution string of the same instant). The fix is millisecond resolution everywhere: SQLite stores via `strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`, the server emits `server_now` in the same format, and the client encodes `since` the same way. With a `.NNN` fragment on every value, lexicographic comparison reduces to numeric comparison.
- **Naming.** `server_seen_at` is slightly misleading because the column also exists on every client-side DB (where nobody delta-queries it across the wire). A more accurate name would be `local_seen_at` or `wrote_at`. Kept the current name to match the wire-protocol terminology; revisit if it confuses future readers.
- **Limitations.** First-time migration of pre-existing rows backfills `server_seen_at` to "now", so legacy rows surface to all peers as a one-time apparent burst on the first sync after upgrade. This is acceptable — peers will merge them as no-ops.
- **Revisit when.** Cross-device clock skew or write-frequency bursts that exceed millisecond resolution become observable; consider option (d).

## Schema versions

Tracked in a new `meta` table per DB (`pragma user_version` is too coarse — we want named keys).

| Version | Date       | Changes                                                                               |
|---------|------------|---------------------------------------------------------------------------------------|
| v0      | pre-ADR    | history: localtime timestamps; wordbank: UTC; no tombstones; no `meta` table          |
| v1      | Phase 1    | both DBs gain `meta(key,value)` table; wordbank table created if absent; history table created in legacy localtime shape (idempotent guard for pre-existing DBs) |
| v2      | Phase 1    | history rows normalised from localtime-with-Z-suffix to true UTC RFC3339; both DBs gain `deleted_at` tombstone column; history gains `i_count` / `i_latest` indexes; wordbank gains `i_words_update_time` |
| v3      | Phase 5/6  | both DBs gain `server_seen_at DATETIME` (nullable due to SQLite ALTER constraint, backfilled in same migration) plus `i_*_server_seen_at` index; runtime writes use `strftime('%f','now')` for millisecond resolution; sync filter switches from `update_time` to `server_seen_at` (see D11) |
| v4      | Round-3 fix | rebuild `words` and `history` tables so column DEFAULTs match the canonical RFC3339-ms format documented in `schema.sql`. Earlier migrations (v1 wordbank `CURRENT_TIMESTAMP`, v1 history `datetime('now','localtime')`, v3 ALTER ADD COLUMN with no default at all) left the on-disk shape inconsistent with the documentation. Done via `CREATE TABLE *_new ... INSERT INTO *_new SELECT FROM ... DROP ... RENAME` since SQLite cannot change DEFAULTs in place. Indexes recreated against the new table. |

Migrations are **idempotent and forward-only**. The migration helper checks `meta.schema_version`, applies the delta, and bumps the version atomically.

## Wire protocol summary (informative)

All timestamps on the wire are RFC3339 UTC with millisecond precision
(`YYYY-MM-DDTHH:MM:SS.sssZ`). See D11 for why second-resolution is unsafe
for the `since` cursor.

```
POST /sync/v1/wordbank/pull
  request : { "since": "2026-06-04T12:00:00.000Z" }   // empty string => full snapshot
  response: { "items": [ { "word": "...", "create_time": "...",
                           "update_time": "...", "deleted_at": null }... ],
              "server_now": "2026-06-04T12:34:56.789Z" }

  Server captures `cutoff = now_ms` BEFORE running ListChanged so any
  push whose server_seen_at lands after cutoff is excluded from this
  response and surfaces on the next pull. The query is
  `WHERE server_seen_at > since AND server_seen_at <= cutoff`; the
  response's `server_now` is the captured cutoff, not a fresh `now()`
  read after the query (see code-review P1b).

POST /sync/v1/wordbank/push
  request : { "items": [ ... ] }
  response: { "applied": 12, "conflicts": [] }

POST /sync/v1/history/pull
  request : { "since": "..." }
  response: { "items": [ { "word": "...", "count": 7,
                           "create_time": "...", "update_time": "...",
                           "deleted_at": null }... ],
              "server_now": "..." }

POST /sync/v1/history/push
  request : { "items": [ ... ] }
  response: { "applied": 8 }

GET  /sync/v1/state
  response: { "server_now": "...", "schema_version": 1 }
```

All requests are authenticated via HTTP Basic Auth. Per-user data lives at
`<config>/sync/<user>/wordbank.db` and `<config>/sync/<user>/history.db`.

## Interaction overview (server ↔ clients)

### Roles

- **Server.** A long-running ondict instance (home server / VPS / NAS) launched with `-sync-server`. Mounts `/sync/v1/*` on the existing Gin server. Per-user SQLite at `<dataDir>/<user>/{wordbank,history}.db`. No master/slave hierarchy — the server is just a convergence point.
- **Clients.** Desktop (`ondict sync`), Android (`mobile.Sync()`), or any future ondict instance. Each client carries its own local SQLite (the same DBs the rest of ondict already uses) and is the source of truth for its own writes between syncs.

### Authentication

Every HTTP call carries `Authorization: Basic base64(user:pass)`. The server compares both sides with `crypto/subtle.ConstantTimeCompare`. Failures return `401` with `WWW-Authenticate: Basic realm="ondict-sync"`. Single user pair (D4).

### Row shape recap

Each wordbank/history row carries:

- `update_time` — user-content time, drives merge winner (D2 last-writer-wins).
- `server_seen_at` — local-write wall-clock instant (millisecond precision), used as the sync delta filter (D11).
- `deleted_at` — tombstone (D6); empty/NULL = live row.
- `count` — history only; merged with `MAX` (D3).

### One `Sync()` cycle = 4 HTTP calls (pull-then-push, wordbank then history)

```
Client                                                       Server
──────                                                       ──────
local SQLite                                                 <user>/wordbank.db
local cursors (in client's meta table):                      <user>/history.db
  sync.last_pull.{wordbank,history}
  sync.last_push.{wordbank,history}

  ── pull wordbank ────────────────────────────────────────►
  POST /sync/v1/wordbank/pull
  { "since": "<last_pull.wordbank>" }       // empty = full snapshot
                                              ┌─────────────────────────────────┐
                                              │ ListChanged((since, cutoff])    │
                                              │ cutoff captured BEFORE the scan │
                                              │ → every row (incl. tombstones)  │
                                              │   the server has observed       │
                                              │   strictly since the cursor and │
                                              │   up to the captured cutoff     │
                                              └─────────────────────────────────┘
  ◄────────────────────────────────────────────────────────
  { "items":[…], "server_now":"<T_now_ms>" }

  syncmerge.MergeWordbank(local_db, in_mem(items))   // LWW + tombstone propagation
  cursor.set("last_pull.wordbank", server_now)

  ── push wordbank ────────────────────────────────────────►
  POST /sync/v1/wordbank/push
  { "items": [rows where local.server_seen_at > last_push] }
                                              ┌────────────────────────────┐
                                              │ syncmerge.MergeWordbank(   │
                                              │   server_db, in_mem(req))  │
                                              │ Upsert stamps              │
                                              │   server_seen_at = now_ms  │
                                              └────────────────────────────┘
  ◄────────────────────────────────────────────────────────
  { "applied": N, "stats": {…} }

  cursor.set("last_push.wordbank", max(pushed.server_seen_at))

  ── pull history  ────────────────────────────────────────► (identical shape)
  ── push history  ────────────────────────────────────────► (identical shape)
```

### Key properties

1. **Both ends run the same merge engine.** `syncmerge.MergeWordbank` / `MergeHistory` are pure functions; the server uses them on push, the client uses them on pull. Re-running a sync is idempotent — second pass only increments `Unchanged`.

2. **Pull-then-push order matters.** Pull first so the client sees peers' state, then push the residual delta. Avoids amplifying round-trip work.

3. **Two cursors per resource, both stored client-side.**
   - `last_pull.*` ← `server_now` from the response (the cutoff the server captured BEFORE scanning); next `since`. Captured-up-front semantics make this race-free against concurrent pushes.
   - `last_push.*` ← max **server_seen_at** of locally-pushed rows (NOT update_time). Push cursors must live in the same time domain ListSince filters on, otherwise an inbound row with a future or skewed update_time could permanently strand subsequent local writes (code-review P1a).
   Stored in the client's wordbank.db `meta` table so they survive process restarts.

4. **Polling, not push.** Server never initiates traffic. Clients poll on a schedule (`ondict sync --loop 10m`, Android WorkManager, or manual). Trade-off: simple + offline-tolerant, but updates lag by up to one polling interval. WebSocket / SSE push is a D5 follow-up.

5. **Convergence is unconditional.** Concurrent edits across devices land at the server, server applies LWW (D2), straggling clients pull the winner on their next cycle and re-apply LWW locally. Final state is independent of operation order.

6. **Deletes flow through the same channel.** `Remove()` writes a tombstone (`deleted_at` set, row stays). The tombstone is just another row to ListSince and just another `Upsert` on the other side. `--gc-tombstones-after` periodically purges older tombstones (D6).

### Worked example: three devices

```
T0   Phone:   Add("alpha")            → phone.wb { alpha live }
T0+  Phone:   Sync()                  → server.wb { alpha }
T1   Laptop:  Sync()                  → laptop.wb { alpha }
T2   Laptop:  Add("beta")             → laptop.wb { alpha, beta }
T2+  Laptop:  Sync()                  → server.wb { alpha, beta }
T3   Phone:   Remove("alpha")         → phone.wb { alpha [tombstone], (no beta yet) }
T3+  Phone:   Sync()
       pull   → server returns beta (server_seen_at @ T2+ > phone.cursor @ T0+)
              merge → phone.wb { alpha [tombstone], beta live }
       push   → server receives alpha tombstone
                (update_time @ T3 > server.alpha.update_time @ T0  → tombstone wins)
              merge → server.wb { alpha [tombstone], beta live }
T4   Laptop:  Sync()
       pull   → server returns alpha tombstone
              merge → laptop.wb { alpha [tombstone], beta live }
       push   → nothing new
```

All three devices converge to `{ alpha [tombstone], beta live }` regardless of operation interleaving.

## Open follow-ups (tracked, not blocking v1)

- D3 follow-up: append-only history event log for true cross-device SUM frequencies.
- D4 follow-up: multi-user accounts; OAuth/OIDC; per-device API tokens.
- D5 follow-up: gRPC or WebSocket push for real-time invalidation.
- D6 follow-up: automated tombstone GC with last-seen-by-client bookkeeping.
- D7 follow-up: full call-site migration to injected `WordbankStore`.
- D10 follow-up: built-in TLS via `autocert`; hosted multi-tenant deployment.
- Encryption-at-rest for sync DBs.
- Quotas / rate-limiting on sync endpoints.

## References

- `history/auto.go` — current history writer implementations and Review path.
- `wordbank/wordbank.go` — current wordbank package-level functions.
- `internal/httpserver/server.go` — HTTP handlers for `/words`, `/dict`, `/search`.
- `mobile/mobile.go:63` — mobile history disabled (to be re-enabled in Phase 7).
- `todo.md:94` — original sync TODO ("A sync-able history system, maybe basicAuth is needed").
- `schema.sql` — reference schema (currently drifts from runtime; reconciled in Phase 1).
- `docs/ARCHITECTURE.md` — broader rendering / Android architecture notes.
- [`docs/sync-deployment.md`](../sync-deployment.md) — deployment & migration runbook for operators (Chinese).
