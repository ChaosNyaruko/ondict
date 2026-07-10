# Ondict Architecture & Design Research

This document covers architectural decisions, ongoing research, and future directions
that are too detailed for AGENTS.md but important to preserve for future work.

---

## Dictionary Entry Rendering

### Current approach

The server renders MDX entry HTML by:
1. Looking up the word in the SQLite `vocab.db` (fast-path) or live MDX decoder (first run).
2. Running it through `render/HTMLRender` which rewrites custom URL schemes
   (`entry://`, `sound://`, `@@@LINK`) into HTTP paths the browser can follow.
3. Injecting all `*.css` files from `dicts/` as a single `<style>` block
   (`sources/mdx.go: initAllCss`).
4. Embedding the result inside the `<article class="entry-card">` in `dict.html`.

Android reuses the lookup and `render/HTMLRender` layers, but not the HTTP page
shell. `mobile.QueryEntry()` requests the `raw` link format and returns only the
rendered entry fragment. Kotlin wraps that fragment in a minimal document with
the dictionary CSS and loads it into an entry-only `WebView`; search, navigation,
word bank, import, and sync remain native views.

---

## MDX Custom URL Schemes

These are **de facto conventions** established by the MDict format, reverse-engineered
and adopted by GoldenDict and others. There is no official spec.

| Scheme | Meaning | Ondict handling today |
|---|---|---|
| `@@@LINK=word` | Entire entry body is a redirect to another headword | `util.ReplaceLINK()` rewrites to an HTML anchor → `/dict?query=word` |
| `entry://word` | Cross-reference link inside entry HTML | HTTP renders rewrite it to `/dict?...`; Android `raw` renders preserve it for native interception |
| `sound://file.mp3` | Audio playback; file lives in the `.mdd` archive | MDX renders convert it to `data-audio-src`; HTTP and Android load the MDD bytes through their respective resource handlers |
| `bres://dict/file` | Bundled resource (image/CSS) from MDD (GoldenDict convention) | Not used in ondict; ondict serves MDD resources on `/filename` paths |

---

## CSS in MDX Dictionaries

CSS files are shipped alongside the `.mdx`/`.mdd` by the dict maker. Key points:

- **Icon fonts are embedded as base64 data URIs** inside the CSS (e.g. the `icomoon`
  font in `LM5style.css` contains the speaker glyph `\ea27`). No separate font files needed.
- `LM5style.css` = entry layout + GoldenDict popup UI + embedded icon fonts.
- `LM5style_vanilla.css` = entry layout + full Longman website shell CSS. Designed for browser rendering.
- Both files are needed together: vanilla for layout, non-vanilla for icon fonts.
- Ondict concatenates all `*.css` files in `dicts/` (`sources/mdx.go: initAllCss`).
  HTTP rendering injects the result as a `<style>` block; Android fetches it through
  `Mobile.getCSS()` and injects it into the entry-only page.

---

## Android Native Shell + Entry WebView

### Current architecture

The Android app uses a native Kotlin shell and embeds Android System WebView only for
dictionary entry HTML. The normal app path does not start Gin or make localhost HTTP
requests:

```text
MainActivity / native controls
  ├─ Mobile.init(filesDir, cacheDir)       paths, dictionaries, local stores
  ├─ Mobile.complete(prefix, limit)        native autocomplete
  ├─ Mobile.queryEntry(word)               QueryMDX(word, "raw")
  │      └─ render.HTMLRender              entry HTML fragment
  ├─ Mobile.getCSS()                       concatenated dictionary CSS
  └─ Mobile.getFile(path)                  MDD audio/image bytes
                 │
                 ▼
       entry-only Android WebView
```

`MainActivity.buildEntryPage()` wraps the fragment in a minimal document and injects
the CSS returned by Go. `WebView.loadDataWithBaseURL()` uses
`https://ondict.local/` as a stable synthetic origin; no server listens there.

The legacy `mobile.StartServer()` and `OndictServerService` remain available, but the
current activity, query, autocomplete, word-bank, and sync flows use direct gomobile
bindings. Sync talks from the Go sync client to the configured remote server through
`Mobile.initSyncOnly()` / `Mobile.sync()`; it does not require a local HTTP server.

### Navigation, audio, and resources

The Android render path asks `HTMLRender` to preserve `entry://` cross-references by
using the `raw` link format. The entry WebView then keeps all dictionary interactions
inside the app:

- `entry://word` is handled by `WebViewClient` or the `Ondict` JavaScript interface,
  then passed back through `Mobile.queryEntry()`.
- Audio elements expose `data-audio-src`; Kotlin loads the bytes with
  `Mobile.getFile()` and plays them through `MediaPlayer`.
- Image and CSS requests are intercepted by `shouldInterceptRequest()` and resolved
  from MDD data through `Mobile.getFile()`.
- Android back navigation uses WebView history when a cross-reference has created a
  previous entry; otherwise it falls through to the activity back stack.

This design removes localhost round trips and keeps interactive controls native while
still paying the Chromium cost needed for arbitrary MDX HTML and CSS.

### System bars and the IME

The app targets API 36. Android 16 disables
`windowOptOutEdgeToEdgeEnforcement`, so the theme flag is only a compatibility opt-out
on Android 15; it cannot by itself prevent content from drawing behind system UI on
Android 16.

All activities therefore inherit `SystemBarsAwareActivity`. It applies system-bar and
display-cutout insets to `android.R.id.content`, producing the same usable content
bounds as a non-edge-to-edge window and preventing child controls from applying the
same insets twice. New activities must inherit this base class.

`MainActivity` additionally enables `avoidImeOverlap`. While the keyboard is visible,
the base class uses the larger of the navigation-bar and IME bottom insets, keeping the
search input above the keyboard. The handled IME inset is then removed before child
dispatch to avoid duplicate padding.

### How GoldenDict handles this (reference implementation)

**GoldenDict (Qt/Desktop)** uses `QWebEngineView` (Chromium via Qt) and registers
custom scheme handlers so the WebView never makes real network requests:

```cpp
QWebEngineProfile::defaultProfile()->installUrlSchemeHandler("entry", handler);
QWebEngineProfile::defaultProfile()->installUrlSchemeHandler("sound", handler);
// handler extracts audio from MDD or triggers a new lookup and replies with bytes
```

Every WebView framework ships this interception mechanism as a first-class feature
precisely because embedded doc/dict viewers are a canonical use case.

### Platform interception APIs

| Platform | WebView | Interception API |
|---|---|---|
| Android | `WebView` (Blink) | `WebViewClient.shouldOverrideUrlLoading()` |
| iOS/macOS | `WKWebView` | `WKNavigationDelegate.decidePolicyFor` |
| Electron | Chromium | `protocol.registerBufferProtocol("sound://", ...)` |
| Qt (GoldenDict) | `QWebEngineView` | `QWebEngineUrlSchemeHandler` |
| Windows | `WebView2` (Edge/Blink) | `AddWebResourceRequestedFilter` |

---

## Research: Lightweight HTML Renderer (Long-term)

### Motivation

Even the entry-only WebView approach pays the full Chromium engine cost. A lightweight
HTML/CSS renderer that handles only the subset of markup MDX entries use would
eliminate that cost entirely — and would also fix the Markdown output quality problem.

### Why this helps Markdown output too

The current `render/MarkdownRender` works by tokenizing raw HTML strings with
`golang.org/x/net/html` and pattern-matching tags. A proper renderer would produce a
structured IR — e.g. `[]Block` where each block is a typed node (`Heading`, `Sense`,
`Example`, `Audio`, etc.) — that both the HTML and Markdown renderers consume.
The Markdown renderer then becomes a clean tree walker instead of a string munger,
and adding support for new dict types is trivial.

### Pure-Go options (researched May 2026)

There is **no mature, production-ready pure-Go HTML+CSS renderer** suitable for
interactive use. The closest candidates:

| Library | Language | Verdict |
|---|---|---|
| [benoitkugler/webrender](https://github.com/benoitkugler/webrender) | Go | Port of WeasyPrint; outputs to PDF/raster only, not interactive. 42 stars, actively developed but narrow scope. |
| `golang.org/x/net/html` | Go | Just a tokenizer/parser, no layout — what we already use. |
| Various toy engines | Go | Incomplete CSS support, not production-ready. |

Writing a Go HTML+CSS layout engine from scratch is not realistic.

### Recommended C++ option: litehtml

[litehtml](https://github.com/litehtml/litehtml) is the most realistic candidate:
- Renders HTML+CSS to a display list via platform-provided callbacks —
  you implement `draw_text`, `draw_background`, `draw_border` etc. using the native
  drawing API (Canvas on Android, Core Graphics on iOS).
- No JavaScript engine — fine, MDX entry HTML doesn't need JS for rendering.
- Already integrated into Android via JNI by open-source dict apps.
- CSS subset covers everything MDX entries use (block layout, inline styles, fonts, borders).
- Used by Vivaldi's reader mode.

Other C++ candidates ruled out:
- **ultralight**: lightweight WebKit-based, good CSS support, but commercial license.
- **servo** (Rust): too heavy and unstable for embedding.
- **WeasyPrint / wkhtmltopdf**: PDF-oriented, not suitable for interactive use.

### If we go this route

The `render/` package interface (`HTMLRender`, `MarkdownRender`) should stay stable.
Only the implementation changes: both renderers consume the same parsed node tree
instead of raw HTML strings. The Markdown renderer maps:
`Sense → numbered list item`, `Example → blockquote`, `Audio → [🔊 word]`, etc.

The MDX entry HTML is constrained enough that a full CSS engine may not even be
necessary — just mapping known class names (`Sense`, `Example`, `GramExa`,
`ColloBox`, etc.) to typed IR nodes would cover the vast majority of entries. That
could be done by extending the existing `golang.org/x/net/html` tokenizer into a
typed node tree with no external dependency.

---

## Wordbank & History — Merge and Cloud Sync

The full design is captured as a versioned ADR at
[`docs/adr/0001-wordbank-history-sync.md`](adr/0001-wordbank-history-sync.md). This
section is a quick orientation pointer for code-reading.

**Layered architecture:**

```
Consumers
  • CLI: `ondict merge` (offline DB-to-DB merge)
  • CLI: `ondict sync` (one-shot or daemon client sync)
  • HTTP /sync/v1/{wordbank,history}/{pull,push,state}
  • mobile.ConfigureSync / mobile.Sync (gomobile bindings)
        │
        ▼
Store interfaces (pkg `store/`)        — WordbankStore, HistoryStore
        │
        ▼
Backends — sqliteWordbank, sqliteHistory; tombstones via `deleted_at`;
           sync delta via `server_seen_at` (ms-resolution local-write time)
        │
        ▼
Merge engine (pkg `syncmerge/`) — pure: LWW by update_time;
                                  count = MAX (idempotent);
                                  create_time = MIN; tombstone propagation
```

**Schema versioning** is owned by `dbutil/migrate.go`; each DB carries a `meta` table
recording its current `schema_version`. v1→v2 normalises legacy localtime history
rows to UTC and adds tombstones; v2→v3 adds `server_seen_at` for sync deltas.

**Why two timestamp columns?** `update_time` is user-content time and drives the
merge winner (last-writer-wins). `server_seen_at` is local-write wall-clock time
used purely as the sync delta filter so that pushes containing rows with
backdated `update_time` still surface to other devices on the next pull.

**Wire protocol:** HTTP REST + JSON, Basic Auth single-user. See the ADR for the
endpoint shapes and the conscious rejection of CRDTs / gRPC / WebSocket push for
v1.
