# Ondict HTML Renderer — Continuation Prompt

This prompt gives a fresh coding agent full context to continue work on the
ondict HTML rendering pipeline. Branch: `android-port`.

---

## Project Overview

**Ondict** is a Go dictionary app (`github.com/ChaosNyaruko/ondict`) that serves
MDX/MDD offline dictionaries and Longman online via HTTP, CLI, and Neovim.
The rendering pipeline converts raw MDX HTML into clean browser-ready HTML.

Live test server: `https://mini.freecloud.dev`  
Raw entry (no page shell): `/dict?query=WORD&engine=mdx&format=html_fragment`  
Full page: `/dict?query=WORD&engine=mdx&format=html`

---

## What Has Been Done

### 1. NodeHandler Registry

Replaced the old monolithic `dfs`/`modifyHref`/`modifyImgSrc`/`replaceMp3`
with a clean extensible interface:

```go
// render/scheme.go
type NodeHandler interface {
    HandleNode(n *html.Node, ctx RenderContext) (skipChildren bool)
}

type RenderContext struct {
    SourceType   string
    EntryFetcher func(word string) string  // fetches another entry's HTML
}
```

`HTMLRender` walks the DOM once, applying `defaultHandlers` in order:

```go
// render/html.go
var defaultHandlers = []NodeHandler{
    EntryHandler{},
    SoundHandler{},
    ImgHandler{},
    ShowImageHandler{},
}
```

`HTMLRender` has an `EntryFetcher` field wired in `sources/mdx.go`:

```go
render.HTMLRender{
    Raw:          def,
    SourceType:   d.t,
    EntryFetcher: func(w string) string { return QueryMDX(w, "html_fragment") },
}
```

`html_fragment` = same render pipeline but skips `<style>` CSS wrapping.

### 2. The Four Handlers (`render/handlers.go`)

| Handler | Matches | Action |
|---|---|---|
| `EntryHandler` | `href="entry://word[#frag]"` | Rewrites to `/dict?query=word&engine=mdx&format=html[#frag]`; strips `__a` suffix from fragment |
| `SoundHandler` | ANY element with `href="sound://..."` | Converts to `<span>` with `<audio preload="none">` + IIFE click script; preserves all original attrs (class, data-*, title), only removes `href` |
| `ImgHandler` | `<img>` | Promotes `base64="data:..."` attr to `src`; strips `file://`; prefixes relative paths with `/` |
| `ShowImageHandler` | `<a class="ldoce-show-image" base64="key">` | Resolves illustration from another entry server-side via `EntryFetcher`; stores result in `data-img-src` |

### 3. SoundHandler Details — Critical Lessons

**Must match any element, not just `<a>`:**
LDOCE5++ uses `<div class="speaker fa fa-volume-up" href="sound://...">`.
The handler checks for `href="sound://"` on any element type.

**Must preserve original attributes:**
`class="fa fa-volume-up"` is a FontAwesome CSS glyph. Stripping it to
`style="cursor:pointer"` makes the icon invisible. Keep all attrs, only
remove `href`.

**Use `<span>` not `<div>` for the replacement:**
Sound elements live inside inline contexts (`<span class="Head">`).
A block `<div>` inside an inline `<span>` is invalid HTML and breaks layout.

### 4. ShowImageHandler Details

The `<a class="ldoce-show-image" base64="ldoce4188jpg">FRUIT 1</a>` in "apple"
entry points to an illustration in the "fruit" entry.

**Resolution strategy (in order):**

1. Find sibling "see picture at WORD" link in the same `.Sense` span.
   The link may have either `entry://fruit#...` (raw, not yet rewritten by
   `EntryHandler` — DFS ordering means sibling not visited yet) or
   `/dict?query=fruit...` (already rewritten). Handle both.
2. Strip `#fragment` from URL, fetch `/dict?query=fruit&engine=mdx&format=html_fragment`.
3. Parse DOM, find first `.big_pic img`, extract `src` → `/fruit_comp.jpg`.
4. Store in `data-img-src` attribute; remove `href`.
5. Fall back to MDD key derivation: `ldoce4188jpg` → `ldoce4188.jpg` (insert
   dot before last 3 chars).

The `big_pic` div in dict HTML: `<div class="big_pic" onclick="expand_thumb()"><img src="/fruit_comp.jpg"/>` — hidden by CSS `display:none`, shown by dict's `expand_thumb()` JS which we don't run.

### 5. Other Fixes Made

**CSS ordering (`sources/model.go`):**
`LM5style_vanilla.css` had `.ldoceEntry .HWD { display: none }` (website CSS).
`LM5style.css` had `display: inherit` (correct for embedded rendering).
`filepath.WalkDir` returns lexical order → vanilla came last → headword hidden.
Fix: sort `*vanilla*` filenames first in `loadAllCss()`.

**`ParseOptionEnableScripting(true)` (`render/html.go`, `render/mdx.go`, `render/ldoceonline.go`):**
Dict HTML assumes JS is on. `<noscript>` content is tracking pixels that
should be invisible. Use `true` so parser treats `<noscript>` as opaque text.

**Fragment scrolling (`render/html.go`, `internal/httpserver/server.go`, `dict.html`):**
`entry://fruit#fruit__entry_0__a` → split on `#`, strip `__a` suffix, emit
as real URL hash. Server strips fragment from query param before MDX lookup.
Page JS does `scrollIntoView()` to `location.hash` after load.

**`base64` attribute duality:**
- On `<img>`: value is a data URI → promote to `src` (`ImgHandler`)
- On `<a class="ldoce-show-image">`: value is an MDD key name (`ShowImageHandler`)
  Different semantics, same attribute name — handle separately.

**Image lightbox (`dict.html`):**
JS is pure UI. All URL resolution done server-side by Go handlers.
```js
// <img class="ldoce-show-image" src="data:..."> → openModal(el.src)
// <a data-img-src="/fruit_comp.jpg"> → openModal(el.getAttribute('data-img-src'))
// Modal backdrop click → close
```

---

## What Needs to Be Done Next

The goal is to **test and refine against the real LDOCE5++ dict** on the server
with real raw MDX content. Areas to investigate:

### A. Verify ShowImageHandler end-to-end

Currently the live server still shows `data-img-src="/ldoce4188.jpg"` (the
fallback MDD key path, which 404s) instead of `data-img-src="/fruit_comp.jpg"`
(the resolved `big_pic` src). Deploy latest code and verify the `EntryFetcher`
strategy resolves correctly.

Debug tip — CDP script to check:
```js
document.querySelector('[data-img-src]')?.getAttribute('data-img-src')
// should be "/fruit_comp.jpg", not "/ldoce4188.jpg"
```

### B. Handle `data-src-mp3` audio elements

Some LDOCE5++ dict elements use `data-src-mp3="/media/..."` instead of
`href="sound://"` for audio. These currently get no audio wiring.
Example: `<span class="speaker exafile" data-src-mp3="/media/english/exaProns/p008-001975414.mp3">`.
A new `DataSrcMp3Handler` or extending `SoundHandler` to also check `data-src-mp3`.

### C. Handle custom audio elements

Some dicts emit `<audio-gb>`, `<audio-us>` custom HTML elements for
pronunciation. These need either a handler or CSS to render correctly.

### D. Explore other raw MDX patterns

With real dict access, scan entries for other patterns that need handling:
- `bres://` scheme (bundled resource, GoldenDict convention)
- `<ill>`, `<ill-g>` custom illustration tags
- Any other `href` schemes not yet handled

---

## Key Files

```
render/scheme.go                        — NodeHandler interface, RenderContext
render/handlers.go                      — All four handlers + helpers
render/html.go                          — HTMLRender, walk(), defaultHandlers
render/sound_test.go                    — FA class preservation test
render/html_test.go                     — All HTML render tests incl. ShowImage
sources/mdx.go                          — QueryMDX wires EntryFetcher; html_fragment
internal/httpserver/server.go           — Strips #fragment from query param
internal/tmpl/templates/dict.html       — Lightbox UI, fragment scroll
sources/model.go                        — loadAllCss() vanilla-first sort
```

## How to Add a New Handler

```go
// 1. render/handlers.go
type MyHandler struct{}

func (MyHandler) HandleNode(n *html.Node, ctx RenderContext) bool {
    // return false = continue walking children
    // return true  = skip children (use for void elements or when done)
    if !IsElement(n, "sometag", "") {
        return false
    }
    // mutate n.Attr, n.DataAtom, n.Data, insert/remove children
    return false
}

// 2. render/html.go — append to defaultHandlers slice
var defaultHandlers = []NodeHandler{
    EntryHandler{},
    SoundHandler{},
    ImgHandler{},
    ShowImageHandler{},
    MyHandler{},  // ← add here
}
```

Helper functions available in `handlers.go`:
- `setAttr(n, key, val)` — set or add attribute
- `removeAttr(n, key)` — remove attribute
- `attrVal(n, key)` — get attribute value
- `hasClass(n, cls)` — check CSS class membership
- `closestClass(n, cls)` — walk up DOM to find ancestor with class
- `walkNodes(n, visit)` — DFS walk, visitor returns true to stop
- `textContent(n)` — concatenated text of node and descendants
- `wordFromDictHref(href)` — extracts word from `entry://` or `/dict?query=` href

---

## Running Tests

```bash
go test ./...                    # full suite
go test ./render/... -v          # render tests with output
make serve                       # start local server on :1345
```
