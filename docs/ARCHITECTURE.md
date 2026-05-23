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

On Android the same path runs, but the full page (app shell + entry card) is
rendered inside a `WebView` backed by Android System WebView (Chromium).

---

## MDX Custom URL Schemes

These are **de facto conventions** established by the MDict format, reverse-engineered
and adopted by GoldenDict and others. There is no official spec.

| Scheme | Meaning | Ondict handling today |
|---|---|---|
| `@@@LINK=word` | Entire entry body is a redirect to another headword | `util.ReplaceLINK()` rewrites to an HTML anchor → `/dict?query=word` |
| `entry://word` | Cross-reference link inside entry HTML | `render/html.go` rewrites `href` to `/dict?query=word&engine=mdx&format=html` |
| `sound://file.mp3` | Audio playback; file lives in the `.mdd` archive | `render/html.go` rewrites to `/<file>` served by `MddFileHandler` |
| `bres://dict/file` | Bundled resource (image/CSS) from MDD (GoldenDict convention) | Not used in ondict; ondict serves MDD resources on `/filename` paths |

---

## CSS in MDX Dictionaries

CSS files are shipped alongside the `.mdx`/`.mdd` by the dict maker. Key points:

- **Icon fonts are embedded as base64 data URIs** inside the CSS (e.g. the `icomoon`
  font in `LM5style.css` contains the speaker glyph `\ea27`). No separate font files needed.
- `LM5style.css` = entry layout + GoldenDict popup UI + embedded icon fonts.
- `LM5style_vanilla.css` = entry layout + full Longman website shell CSS. Designed for browser rendering.
- Both files are needed together: vanilla for layout, non-vanilla for icon fonts.
- Ondict concatenates all `*.css` files in `dicts/` and injects them as a single
  `<style>` block per entry (`sources/mdx.go: initAllCss`).

---

## Research: Native Android Rendering

### Why the current full-WebView approach feels heavy

The Android app currently loads the entire Go HTTP server response (app shell + entry
card) into a `WebView`. The WebView engine (Android System WebView / Chromium) is
loaded regardless, so there is no memory saving from this approach — but the UX
suffers because native UI elements (search bar, navigation) feel slower inside a
WebView than as native Kotlin views.

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

### Recommended Android approach (future work)

The pragmatic middle ground — native shell, entry-only WebView, scheme interception —
gives most of the UX benefit without a full renderer rewrite:

1. Keep the native Kotlin UI (search bar, navigation, word bank) fully native.
2. Use a `WebView` scoped to just the `<article class="entry-card">` region.
3. Load entry HTML directly (`webView.loadDataWithBaseURL(...)`) instead of making
   an HTTP request to the local Go server.
4. Register a `WebViewClient` and override `shouldOverrideUrlLoading` /
   `shouldInterceptRequest`:
   - `entry://word` → call back into Go (`sources.QueryMDX`) and reload the WebView.
   - `sound://file.mp3` → call `sources.GetMDDFile(filename)` and play with `MediaPlayer`.
   - CSS/image resources → serve from MDD via `shouldInterceptRequest` returning a
     `WebResourceResponse`.
5. Inject the concatenated CSS directly into the HTML string before loading
   (same as current `allCss` approach) rather than relying on `<link>` tags.

This eliminates the local HTTP server dependency on Android for the query/render path,
while reusing all existing Go rendering logic via gomobile bindings.

**What this saves vs. what it doesn't:**
- Saves: localhost HTTP round-trip latency, Gin server overhead, native UI responsiveness.
- Does not save: the Chromium engine cost — Android System WebView is still loaded.

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
