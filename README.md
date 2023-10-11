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

### Examples
#### One-shot query
```console
ondict -q <word>
```

#### One-shot query, but from remote server
```console
ondict -q <word> -remote auto 
```

#### A "repl" querier
```console
ondict -i
```
input `.help` for commands that can be used.

#### Work as a server
This app can also serve as a HTTP server, allowing remote fetch and query, with cache and acceleration.
```console
ondict -server
```

## Working with Neovim
1. Install the plugin with a plugin manager or manually. 
2. Use `:lua require("ondict").query()` to query \<cword\>.
3. Define a mapping for yourself to call it easier. NOTE: in visual mode, use "\<cmd\>lua require("ondict").query()\<cr\>" instead. It will capture the "SELECTED" word. Otherwise, the "mode" will be changed and only "\<cword\>" can be queried.

Install the "ondict" binary automatically with [lazy](https://github.com/folke/lazy.nvim).
```
{ 
    "ChaosNyaruko/ondict",
    event = "VeryLazy",
    build = function(plugin)
        require("ondict").install(plugin.dir)
    end
}
```

Manually
```console
cd ~/.local/share/nvim/site/pack/packer/start/
git clone https://github.com/ChaosNyaruko/ondict.git
cd ondict
go install .
```
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
- [x] Basic parsing from Longman online dictionary.
- [x] Do HTTP req instead of parsing a static html file.
- [x] Integrated with (n)vim.
- [x] Hyphen-connected for phrases, and "space separated" queries.
- [x] Take input from stdin.
- [x] Work as a server (to cache something).
- [x] Cache and save/restore stuff, in pure text.
- [x] A real "auto" mode.
- [x] Kill the server with a timeout.
- [ ] A system for reviewing, e.g. simple ANKI?
- [ ] More information such as collocations/corpus/.....
- [x] Format: basic colors.
- [ ] Format: indents and blank lines.
- [ ] Serve on a TCP connection and can query from a "real" remote server, rather than local UDS.
---
- [ ] ~Other dict parsing engines if I have the motivation?~
- [ ] ~offline support? Not going to do that recently... I just use it myself.~
- [ ] ~Vim version~
- [ ] a simple TUI using https://github.com/charmbracelet/bubbletea ?

