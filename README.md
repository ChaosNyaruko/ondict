# Introduction
It's a really simple dictionary CLI app, relying on Longman online dictionary, with simple cache and history functionality.

![Gif](./assets/ondict_example1.gif)
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
ondict -server -listen=localhost:1345 -engine=mdx
```
Launch a http request
```console
curl "http://localhost:1345/?query=apple&engine=mdx&format=x"
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
# Offline dictionary files
Put the decoded JSON files in $HOME/.config/ondict/dicts

# Features
- Online query support based on [Longman online dictionary](https://ldoceonline.com)
- Integrated with (n)vim, feel free to use it in whatever editor you are using!
- Offline engine/mode is supported. The online engine may be more comprehensive and updated, but they are slow since an HTTP request is made for the first time.
