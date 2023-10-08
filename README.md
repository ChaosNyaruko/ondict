# Introduction
It's a really simple dictionary CLI app, relying on Longman online dictionary, with simple cache and history functionality.

![Gif](./assets/ondict_example.gif)
# Prerequisites
- Go version >=1.16, and add $GOBIN in your $PATH
- Neovim version >= 0.9.1 [recommended, cause I developed it on this version, but previous versions may also use it, only some "lsp" utils (not lsp feature itself) is required. So it can also be ported to Vim, but I am not quite familiar with vim's popup feature]
# Installation
```console
go install github.com/ChaosNyaruko/ondict@latest
```
# Usage
## CLI
### Help
```console
ondict 
```
```console
ondict -h
```
### One-shot query
```console
ondict -q <word>
```

### A "repl" querier
```console
ondict -i
```
input `.help` for commands that can be used.

### Work as a server
This app can also serve as a HTTP server, allowing remote fetch and query, with cache and acceleration.
```console
ondict -s
```

## Working with Vim
1. Install the plugin with a plugin manager or manually.
2. Use `:lua require("ondict").query()` to query \<cword\>.
3. Define a mapping for yourself to call it easier. NOTE: in visual mode, use "\<cmd\>lua require("ondict").query()\<cr\>" instead. It will capture the "SELECTED" word. Otherwise, the "mode" will be changed and only "\<cword\>" can be queried.

### Mapping examples
```vimscript
nnoremap <leader>d <cmd>lua require("ondict").query()<cr>
vnoremap <leader>d <cmd>lua require("ondict").query()<cr>
```

```lua
vim.keymap.set("n", "<leader>d", require("ondict").query)
vim.keymap.set("v", "<leader>d", require("ondict").query)
```


# Features
- [x] basic parsing from Longman online dictionary
- [x] do HTTP req instead of parsing a static html file
- [x] integrated with (n)vim
- [x] hyphen-connected for phrases, and "space separated" queries.
- [x] take input from stdin
- [x] work as a server (to cache something)
- [x] cache and save/restore stuff, in pure text
- [x] a real "auto" mode
- [ ] kill the server with a timeout
- [ ] a system for reviewing, e.g. simple ANKI?
- [ ] more information such as collocations/corpus/...
- [x] format: basic colors
- [ ] format: indents 
- [ ] a simple TUI using https://github.com/charmbracelet/bubbletea ?

---
- [ ] ~Other dict parsing engines if I have the motivation?~
- [ ] ~offline support? Not going to do that recently... I just use it myself.~
- [ ] ~Vim version~

