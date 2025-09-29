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

### Go Standards
- **Go version**: 1.23.0+ with toolchain 1.23.9
- **Package structure**: Clear separation between `decoder`, `sources`, `render`, `util`, `history`
- **Error handling**: Uses `logrus` for structured logging with debug/info levels
- **Interface design**: Well-defined interfaces (`RawOutput`, `Searcher`, `Source`)

### Frontend Guidelines
- **Pure HTML/CSS/JavaScript**: No complex frameworks
- **Template constants**: All HTML/CSS embedded in `template.go` for portability
- **Minimal dependencies**: Simple, maintainable front-end code

### Naming Conventions
- **Packages**: Lowercase, descriptive (e.g., `decoder`, `sources`, `render`)
- **Types**: PascalCase with clear purpose (e.g., `MdxDict`, `DictConfig`)
- **Functions**: MixedCase for exported functions, camelCase for internal
- **Variables**: Descriptive names, avoid single letters except in loops

## Testing Framework

### Test Structure
- **Unit tests**: `*_test.go` files alongside implementation
- **Test files**: `decoder/mdx_test.go`, `sources/model_test.go`, `util/utils_test.go`
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