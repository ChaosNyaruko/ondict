# Prerequisites
- Go version >=1.16
# Installation
```console
go install github.com/ChaosNyaruko/ondict@latest
```
# Usage
## CLI
### One-shot query
```console
ondict -q <word>
### A repl-like querier

```console
ondict -i
```
## Working with Vim
1. Install the plugin with a plugin manager or manually.
2. Use `:lua require("ondict").query()` to query \<cword\>.
3. Define a mapping for yourself to call it easier. NOTE: in visual mode, use "\<cmd\>lua require("ondict").query()\<cr\>" instead. It will capture the "SELECTED" word. Otherwise, the "mode" will be changed and only "\<cword\>" can be queried.

# TODO
- [ ] this README document
- [x] parsing
- [x] do HTTP req instead of parsing a static html file
- [x] integrated with (n)vim
- [ ] hyphen-connected for phrases, and "space separated" queries.
- [x] take input from stdin
- [ ] work as a server (to cache something)
- [ ] cache and save/restore stuff, in pure text, for reviewing, i.e. simple ANKI?
- [ ] more information such as collocations/corpus/...
- [ ] format: indents and colors(go get github.com/fatih/color go get github.com/mattn/go-colorable)
- [ ] a simple TUI using https://github.com/charmbracelet/bubbletea ?

    ---
- [ ] ~Other dict parsing engines if I have the motivation?~
- [ ] ~offline support? Not going to do that recently... I just use it myself.~

