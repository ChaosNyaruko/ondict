# Where queries land — data flow reference

> Developer-facing reference. End users don't need to read this; they just want
> "my Word Bank stays in sync between my phone and laptop". This doc is for
> ondict contributors who need to reason about which `*.db` file gets written
> in which invocation mode.

The companion docs are:
- [`adr/0001-wordbank-history-sync.md`](adr/0001-wordbank-history-sync.md) — design decisions
- [`sync-deployment.md`](sync-deployment.md) — operator runbook (Chinese)

## The three places data can live

| Tag | Path | Read / written by |
|---|---|---|
| **Local wordbank** | `<config>/wordbank.db` | This process's ondict, via `wordbank.Add/Remove/Contains/List` (singleton-backed). |
| **Local history** | `<config>/history.db` + `<config>/history.table` | Same; `*history.History.Append` fans out to the SQLite writer + the txt log. |
| **Sync user library** | `<sync-data-dir>/<user>/{wordbank,history}.db` | Only the sync server (`-sync-server`) writes here, and only when a client `Sync()` call hits the `/sync/v1/*/push` endpoints. **Never** read from local query handlers. |

`<config>` resolves to:
- Desktop: `~/.config/ondict/`
- Android: `filesDir` (set via `util.SetPaths` from `mobile/mobile.go: StartServer`).

`<sync-data-dir>` defaults to `<config>/sync` and can be overridden with
`-sync-data-dir`.

## The invariant to keep in mind

> Whichever process performs a query, the **first landing place is always
> that process's own `<config>/{wordbank,history}.db`**. To make the data
> show up in the sync server's user library, **some** process must
> explicitly invoke a sync client cycle (CLI `ondict sync` or
> `mobile.Sync()`) — the sync server does **not** snoop on its own host's
> `<config>` and does **not** automatically count it as alice's data.

This is why the architecture works offline: queries write to a local SQLite
that doesn't depend on the network. Sync is a separate, explicit, idempotent
verb you invoke when you want.

## Three invocation modes

### 1. Server itself queries via HTTP

> e.g. on the same box that's running `ondict -serve -listen=:1345 -sync-server`,
> someone points a browser or `curl` at `http://localhost:1345/dict?...`.

Code path: `internal/httpserver/server.go: queryWord` (or `searchHandler`).
- `wordbank.Contains(word)` runs to render the "Add to Word Bank" button state — read-only
- If the request has `record=1`, `his.Append(word)` writes the history row

Lands in:
- ✅ **Local history** (`<config>/history.db` + `<config>/history.table`) when `record=1`
- ❌ Local wordbank — **only** if the user clicks the "Add" button (a separate POST `/words/add` → `wordbank.Add`); plain queries don't auto-add
- ❌ Sync user library — **not written** by this path, even when `-sync-server` is enabled on the same process

The third row is the gotcha: **enabling `-sync-server` does not implicitly
sync the host machine's own queries**. To get that, run `ondict sync
--base-url http://127.0.0.1:1345` on the same host (one-shot or `--loop`),
which acts like any other client and pushes `<config>/{wordbank,history}.db`
through the wire protocol.

### 2. Server runs `ondict -q xxx` with empty `-remote`

> Pure one-shot CLI on the server box. No UDS forwarding because `-remote`
> is empty.

Code path: `main.go` one-shot branch.

```go
his = history.NewHistory(history.NewTxtWriter(), history.NewSqlite3Writer())
...
fmt.Println(query(*word, *engine, *renderFormat, *record&0x1 != 0))
```

`query()` invokes `his.Append(word)` only when `r == true` (i.e. `-r 1` or
any flag with the low bit set).

Lands in:
- ✅ **Local history** when `-r 1` is supplied
- ❌ Local wordbank — `-q` never touches `wordbank.*`
- ❌ Sync user library — same as scenario 1

If `-r` is absent or 0 (the default), even local history is skipped — this
matches the long-standing CLI behavior; we kept it for backward
compatibility.

### 3. Android (WebView + in-process server) — and how its data reaches the real remote server

> The Android app embeds the Go server in-process bound to `127.0.0.1:1345`,
> with a WebView pointing at it. `<config>` is redirected to `filesDir` via
> `util.SetPaths` in `mobile/mobile.go: StartServer`.

#### Phase A — query lands locally

Same code path as scenario 1 (the WebView is just an HTTP client).
Difference: `<config>` is on Android private storage, and history is
re-enabled (`mobile/mobile.go: StartServer` now passes
`History: history.NewHistory(history.NewSqlite3Writer())` after Phase 7;
ADR D9). So:

- ✅ Android-local wordbank (`filesDir/wordbank.db`) when the user taps "Add"
- ✅ Android-local history (`filesDir/history.db`) on every recorded query
- ❌ Remote server — **not** touched by the query itself

#### Phase B — sync to remote

The Activity invokes the gomobile binding:

```kotlin
mobile.Mobile.configureSync(baseURL, user, pass)
val summary = mobile.Mobile.sync()  // one pull-then-push cycle
```

`Sync()` internally (per resource: wordbank then history):

1. Read cursor from the device's local `<config>/wordbank.db`'s `meta` table:
   `sync.last_pull.<resource>`
2. `POST /sync/v1/<resource>/pull { since }` to remote
3. `syncmerge.Merge<Resource>` writes returned rows into the local DB
4. Update local cursor `sync.last_pull.<resource>` to the response's `server_now`
5. `ListSince(local server_seen_at > sync.last_push.<resource>)` collects rows to push
6. `POST /sync/v1/<resource>/push { items }`
7. Server runs the same merge engine against `<sync-data-dir>/<user>/<resource>.db`
8. Update local cursor `sync.last_push.<resource>` to `max(SeenAt of pushed rows)`

Lands in:
- ✅ Android-local DBs — every query, immediately
- ✅ **Remote sync user library** (`<sync-data-dir>/<user>/...`) — only when `Sync()` is invoked

The "query writes locally + sync writes remote on demand" split is what
makes the Android app work offline: query handlers don't reach for the
network.

## Reference table

| Mode | Local wordbank<br/>(`<config>/wordbank.db`) | Local history<br/>(`<config>/history.db` + `.table`) | Remote sync user library<br/>(`<sync-data-dir>/<user>/`) |
|---|:---:|:---:|:---:|
| **1. Server self-query (HTTP)** | only if "Add" button clicked | ✅ when `record=1` | ❌ unless host also runs `ondict sync` |
| **2. Server `ondict -q` (no remote)** | ❌ | ✅ when `-r 1` | ❌ |
| **3. Android WebView + Sync()** | only if "Add" button clicked | ✅ (re-enabled in Phase 7) | ✅ on `Sync()` invocation only |

## Common operator confusion

- **"I enabled `-sync-server` on the same box where I query — why isn't my
  local data showing up on my phone?"** Because the server doesn't auto-pull
  from `<config>`. Either run `ondict sync --base-url http://127.0.0.1:1345`
  on the host (so it behaves as a client of itself), or do an offline
  bootstrap with `cp` / `ondict merge` per
  [`sync-deployment.md` §4](sync-deployment.md).
- **"I want my phone to broadcast every query immediately."** The polling
  model is intentional (offline tolerant, no WS server). For lower latency,
  schedule `mobile.Sync()` more often — every minute is fine; the engine is
  idempotent and a no-op on the server side when there's nothing new.
- **"Do `ondict -q` queries get synced?"** Only after the host process also
  runs `ondict sync` against some sync server. The CLI's `-r` writes purely
  local — there is no implicit network call.

## Code references

- `internal/httpserver/server.go:106` — `queryWord` (HTTP query handler)
- `main.go:265,283` — CLI `-q` recording branches
- `mobile/mobile.go:StartServer` — Android wiring; `History` is now non-nil
- `mobile/mobile.go:ConfigureSync` / `Sync` — gomobile bindings
- `internal/syncclient/client.go:Sync` — pull-then-push cycle
- `internal/syncserver/server.go:handle*Push` — server-side merge entry
