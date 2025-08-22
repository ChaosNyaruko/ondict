```
# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.
```
## Common Commands
- **Install**: `go install github.com/ChaosNyaruko/ondict@latest` or `git clone https://github.com/ChaosNyaruko/ondict.git && make install`
- **Run HTTP server**: `ondict -serve -listen=localhost:1345 -e=mdx` (use `make serve` for local example)
- **One-shot query**: `ondict -q <word> [-e mdx]`
- **Remote query**: `ondict -q <word> -remote localhost:1345`

## High-Level Architecture
- **Core**: Written in Go, supporting CLI, HTTP server, and Neovim integration.
- **Key Features**: MDX dictionary parsing, online Longman querying, UNIX stdout compliance for CLI workflows.
- **Configuration**: Stored in `~/.config/ondict` (config.json, dicts directory for offline dictionaries).

## Development
- Generate certain unit tests if you can. 
- When we have to deal with some front-end stuff(HTML/CSS/JavaScript/...):
  - DO NOT use complicated front-end framework, simple pure HTML/CSS/JavaScript is preferred.
  - The modification should take place in the "const" variables in `template.go`, so we can run the server at any directory, without having to copy the HTML files and resources.
  - I'm not so good at front-end, so if advance/complex CSS feature is used, explain them please.

## Important Notes
- **Installation**: Requires Go >=1.16 and Neovim >=0.9.1 (for integration).
- **Offline Dictionaries**: Place MDX/MDD files in `~/.config/ondict/dicts`.
- **Neovim Integration**: Use lazy.nvim or manual clone; map `<leader>d` to `require("ondict").query()`.
- **Web Mode**: Recommended for full HTML rendering; access via browser at `localhost:1345`.

