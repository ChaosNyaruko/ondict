# Ondict Architecture Overview

## Project Overview

Ondict is a Go-based dictionary application that supports both online and offline dictionary queries. It provides multiple interfaces including CLI, HTTP server, and Neovim integration. The application specializes in MDX dictionary format parsing and Longman online dictionary integration.

**Key Architecture Components:**
- **Multi-mode operation**: CLI one-shot queries, interactive REPL, HTTP server, and remote querying
- **Dual engine support**: MDX offline dictionaries and Longman online dictionary
- **Multiple output formats**: HTML (web mode) and Markdown (CLI/TUI mode)
- **Plugin ecosystem**: Neovim integration, FZF support, Hammerspoon automation

## Build & Commands

### Installation
```bash
# From source (recommended)
go install github.com/ChaosNyaruko/ondict@latest

# Or clone and build
git clone https://github.com/ChaosNyaruko/ondict.git
make install
```

### Development Commands
```bash
# Run HTTP server (web mode)
make serve                    # Local development server
make serve-v                 # Verbose mode

# One-shot queries
make run word=doctor         # Query specific word
make local                   # Test with apple
make mdx word=test           # MDX engine query
make query-online word=test  # Online engine query

# Testing
make test                    # Run all tests with coverage
make localtest               # Full test suite with FULLTEST=1

# Build
make build                   # Build with version/commit flags
./build.sh                   # Manual build script
```

### Docker Deployment
```bash
docker build . -t ondict
docker run --rm --name ondict-app --publish 1345:1345 \
  --mount type=bind,source=$HOME/.config/ondict,target=/root/.config/ondict ondict
```

## Code Style & Conventions
### Documentation
- Everytime we add a new feature, remember to update the README.md and the Chinese version.
- Update AGENTS.md if you recognize some general rules in the session.

### Go Standards
- **Go version**: 1.23.0+ with toolchain 1.23.9
- **Package structure**: Clear separation between `decoder`, `sources`, `render`, `util`, `history`
- **Error handling**: Uses `logrus` for structured logging with debug/info levels
- **Interface design**: Well-defined interfaces (`RawOutput`, `Searcher`, `Source`)

### Frontend Guidelines
- **Pure HTML/CSS/JavaScript**: No complex frameworks
- **Embedded templates**: HTML pages live under `templates/` and are embedded via Go `embed` + `template.ParseFS`
- **Minimal dependencies**: Simple, maintainable front-end code

### Naming Conventions
- **Packages**: Lowercase, descriptive (e.g., `decoder`, `sources`, `render`)
- **Types**: PascalCase with clear purpose (e.g., `MdxDict`, `DictConfig`)
- **Functions**: MixedCase for exported functions, camelCase for internal
- **Variables**: Descriptive names, avoid single letters except in loops

## Testing Framework

### Test Structure
- **Unit tests**: `*_test.go` files alongside implementation
- **Coverage**: Integrated coverage reporting with `cover.out` and `cover.html`

### Test Execution
```bash
# Standard test suite
go test ./... -coverprofile=cover.out -v
go tool cover -func cover.out | tail -1
go tool cover -html=cover.out -o cover.html

# Full test suite with real dictionaries
FULLTEST=1 go test -v ./...
```

### Test Data
- **Sample dictionaries**: `testdata/` directory with test MDX files
- **Mock data**: Test dictionary entries and HTML samples
- **Integration tests**: Real dictionary loading when `FULLTEST=1`

## Security Considerations

### Data Protection
- **Local storage**: Dictionary files stored in `~/.config/ondict/dicts/`
- **History tracking**: Optional query history in SQLite database
- **No telemetry**: Application does not send usage data

### Network Security
- **Local server**: Unix domain sockets preferred for local communication
- **Remote queries**: Optional remote server support with timeout controls
- **Session management**: Cookie-based sessions with secret keys

### Input Validation
- **Query sanitization**: Word queries properly handled and escaped
- **File path validation**: Dictionary file paths validated before loading
- **Template rendering**: HTML templates properly escaped

## Configuration Management

### Directory Structure
```
~/.config/ondict/
├── config.json          # Dictionary configuration
├── dicts/               # MDX/MDD dictionary files
├── history.sqlite       # Query history database
└── *.css               # Custom CSS files for dictionaries
```

### Configuration Format
```json
{
  "dicts": [
    {
      "name": "DictionaryName",
      "type": "LONGMAN5/Online",
      "css": "custom.css"
    }
  ]
}
```

### Environment Variables
- **XDG_CONFIG_HOME**: Base configuration directory
- **FULLTEST**: Enable full test suite with real dictionaries

## Key Dependencies

### Core Libraries
- **gin-gonic/gin**: HTTP web framework
- **sirupsen/logrus**: Structured logging
- **ncruces/go-sqlite3**: SQLite database driver
- **fatih/color**: Terminal color output

### Dictionary Processing
- **BobuSumisu/aho-corasick**: Aho-Corasick algorithm for fuzzy search
- **C0MM4ND/go-ripemd**: RIPEMD-128 hash for MDX decryption
- **schollz/progressbar**: Progress indication for dictionary loading

### Testing
- **stretchr/testify**: Test assertions and mocking

## Architecture Patterns

### Lazy Loading
- **Memory efficiency**: Dictionaries loaded on-demand
- **Performance**: Optional full pre-loading for faster queries
- **Configuration**: Controlled via `-lazy` flag

### Plugin Architecture
- **Source abstraction**: Clean interface for different dictionary sources
- **Renderer system**: Separate HTML and Markdown renderers
- **History system**: Pluggable history backends (text, SQLite)

### Concurrent Design
- **Server mode**: Concurrent request handling with Gin
- **Background loading**: Dictionary loading in background threads
- **Timeout management**: Automatic server shutdown on idle timeout

## Frontend Development
- You are a frontend expert, but try NOT to use any bloated frontend framework, use plain and standard HTML/CSS, and as little JavaScript as possible.
- The application is launched independently of the working directory by embedding `templates/*.html` with Go `embed`. When adding frontend features, update the embedded templates and keep the server handlers aligned with those template names.

## Research: Native Dictionary Rendering (Android / Future)

### Background
The current Android app embeds a full `WebView` that loads the Go HTTP server's HTML pages. This works but feels heavy — the WebView renders the entire app shell (nav, search bar, etc.) rather than just the dictionary entry content.

A better long-term architecture is to keep the native Android UI in Kotlin/XML or Compose and use the WebView (or a lighter alternative) **only** for the dictionary entry HTML, with custom URL scheme handling replacing the current HTTP round-trip for `entry://` and `sound://` links.

### How mature apps handle this (GoldenDict reference)

**GoldenDict (Qt/Desktop)** uses `QWebEngineView` (Chromium via Qt) and registers custom scheme handlers:
```cpp
QWebEngineProfile::defaultProfile()->installUrlSchemeHandler("entry", handler);
QWebEngineProfile::defaultProfile()->installUrlSchemeHandler("sound", handler);
// handler extracts audio from MDD or triggers a new lookup and replies with bytes
```

The key insight: every WebView framework ships a first-class interception hook precisely because embedded doc/dict viewers are a canonical use case.

### MDX custom URL schemes (de facto conventions, not standards)

These are conventions established by the MDict format, reverse-engineered and adopted by GoldenDict and others. There is no official spec.

| Scheme | Meaning | Ondict handling today |
|---|---|---|
| `@@@LINK=word` | Entire entry body is a redirect to another headword | `util.ReplaceLINK()` rewrites to an HTML anchor → `/dict?query=word` |
| `entry://word` | Cross-reference link inside entry HTML | `render/html.go` rewrites `href` to `/dict?query=word&engine=mdx&format=html` |
| `sound://file.mp3` | Audio playback; file lives in the `.mdd` archive | `render/html.go` rewrites to `/<file>` served by `MddFileHandler` |
| `bres://dict/file` | Bundled resource (image/CSS) from MDD (GoldenDict convention) | Not used in ondict; ondict serves MDD resources on `/filename` paths |

### CSS in MDX dictionaries

CSS files (e.g. `LM5style_vanilla.css`, `LM5style.css`) are shipped alongside the `.mdx`/`.mdd` by the dict maker and are meant to be applied when rendering entry HTML. Key points:

- **Icon fonts are embedded as base64 data URIs** inside the CSS (e.g. the `icomoon` font containing the speaker glyph `\ea27`). This makes the CSS self-contained — no separate font files needed.
- `LM5style.css` = entry layout + GoldenDict popup UI + embedded icon fonts.
- `LM5style_vanilla.css` = entry layout + full Longman website shell CSS (responsive grid, homepage widgets, ads). Designed for browser rendering.
- Both files are needed together for correct rendering (vanilla for layout, non-vanilla for icon fonts).
- Ondict currently concatenates all `*.css` files in `dicts/` and injects them as a single `<style>` block per entry (`sources/mdx.go: initAllCss`).

### Platform interception APIs

| Platform | WebView | Interception API |
|---|---|---|
| Android | `WebView` (Blink) | `WebViewClient.shouldOverrideUrlLoading()` |
| iOS/macOS | `WKWebView` | `WKNavigationDelegate.decidePolicyFor` |
| Electron | Chromium | `protocol.registerBufferProtocol("sound://", ...)` |
| Qt (GoldenDict) | `QWebEngineView` | `QWebEngineUrlSchemeHandler` |
| Windows | `WebView2` (Edge/Blink) | `AddWebResourceRequestedFilter` |

### Recommended Android approach (future work)

1. Keep the native Kotlin UI (search bar, navigation, word bank) fully native.
2. Use a `WebView` scoped to just the `<article class="entry-card">` region.
3. Load entry HTML directly (`webView.loadDataWithBaseURL(...)`) instead of making an HTTP request to the local Go server.
4. Register a `WebViewClient` and override `shouldOverrideUrlLoading` / `shouldInterceptRequest`:
   - `entry://word` → call back into Go (`sources.QueryMDX`) and reload the WebView with the new entry HTML.
   - `sound://file.mp3` → call `sources.GetMDDFile(filename)` and play the bytes with Android's `MediaPlayer`.
   - CSS/image resources → serve from MDD via `shouldInterceptRequest` returning a `WebResourceResponse`.
5. Inject the concatenated CSS directly into the HTML string before loading (same as current `allCss` approach) rather than relying on `<link>` tags that would need separate resource interception.

This eliminates the local HTTP server dependency on Android entirely for the query/render path, while reusing all existing Go rendering logic via gomobile bindings.

