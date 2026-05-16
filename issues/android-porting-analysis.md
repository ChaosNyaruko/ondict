# Android Porting Analysis

Date: 2026-05-11

## Goal

Run Ondict locally on an Android phone with:
1. HTML-like display (web rendering for definition content)
2. Local offline dictionary queries (no internet required)
3. Working internal links: `entry://`, `sound://`, `@@@LINK`

---

## Approach Options

### Option A: Termux (zero code changes)
- Install [Termux](https://termux.dev) + `pkg install golang`
- Build/install `ondict` natively on-device
- Access via `localhost:1345` in a mobile browser
- **Pros**: No code changes; all features work immediately
- **Cons**: Power-user only; not a real installable app

### Option B: Native Android App (WebView + gomobile)
- Compile Go logic to `.aar` via [gomobile](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile)
- Wrap in a Kotlin/Java Android app with a `WebView`
- Serve HTML from the existing Gin HTTP server running as a background service
- Load `http://localhost:<port>` in the WebView
- **Pros**: Real installable APK, seamless UX, reuses all existing server code
- **Cons**: Significant build complexity; gomobile has limitations

### Option C: Go-in-Android Background Service (most practical path to a real app)
Same as Option B but scoped to embedding only the minimal HTTP server, keeping the WebView as the UI layer.

---

## What Needs to Change Per Requirement

### 1. HTML Display
The existing `dict.html` + `HTMLRender` pipeline already produces standard HTML/CSS/JS.
Android `WebView` renders this without modification. **No code changes needed.**

### 2. Local Offline Decoding
`MdxDict` + `decoder.MDict` already decode `.mdx` files entirely locally.

**Issue**: Paths are hardcoded to macOS/Linux conventions in `util/path.go`:
- `~/.config/ondict/dicts/` — dict files
- `~/Library/Caches/ondict/` — TmpDir for dumped MDD resources (audio, images)

**Fix needed**: Make `ConfigPath()` and `TmpDir()` accept overrides (env var or init-time setter) so Android paths like `/data/data/<package>/files/` can be used.

### 3. Internal Links: `entry://`, `sound://`, `@@@LINK`

All three are already handled in web/HTML mode:

| Link type | Handler | Location | Status |
|---|---|---|---|
| `@@@LINK=word` | Rewrites to `/dict?query=word` anchor | `util/render.go:ReplaceLINK()` | Works as-is |
| `entry://word` | Rewrites to `/dict?query=word&engine=mdx&format=html` | `render/html.go:modifyHref()` | Works as-is |
| `sound://file.mp3` | Creates `<audio>` + `<script>` click handler | `render/html.go:replaceMp3()` | Works as-is |

**One gap**: `HTMLRender.Render()` (`render/html.go:32`) only processes links when `SourceType` starts with `"LONGMAN"`. For non-Longman MDX dicts the raw HTML is returned untouched — link rewriting and audio injection are skipped.

**Fix needed**: Apply link rewriting for all MDX source types, not just Longman.

---

## Summary of Required Code Changes

| Area | File | Change |
|---|---|---|
| Path abstraction | `util/path.go` | Make `ConfigPath()` / `TmpDir()` accept runtime overrides |
| SourceType gate | `render/html.go:32` | Apply link rewriting to all MDX sources, not just `LONGMAN*` |
| Build toolchain | new | `gomobile bind` setup + Kotlin/Android wrapper scaffolding |

---

## Recommended Path

1. **Validate with Termux first** — confirm the full flow works on-device with zero code changes
2. **Fix the SourceType gate** in `render/html.go` — low-effort, high-value
3. **Abstract file paths** in `util/path.go` — required for any Android packaging
4. **Scaffold Android app** using `gomobile bind` + a minimal Kotlin `WebView` Activity

The dictionary decoding, HTML rendering, and link handling are all sound. The main work is build toolchain and Android filesystem integration.
