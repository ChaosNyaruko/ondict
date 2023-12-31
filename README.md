# Introduction
Yet another simple dictionary application. Support multiple sources, including Longman online dictionary, and user-loaded MDX/MDD dictionary files.

# Disclaimer
It is trying to be just a **dictionary**, which you may need during your English study or writing some English posts. 

It is _NOT_ a "translator", in which scenario an LLM model based tool is more suitable in modern days.

Web mode is recommended, because both the online engine and MDX engine are based on HTML/CSS stuff. So when you need its output to be rendered as markdown, a independent parser and renderer need to written for each source, that's quite a lot of work and almost impossible.

I just write a simple markdown renderer for [Longman Dictionary of Contemporary English](https://github.com/ChaosNyaruko/ondict/releases/download/v0.0.5/Longman.Dictionary.of.Contemporary.English.mdx) MDX dictionary, which I uploaded in some releases, so that you can roughly use its markdown rendered output in some cases, such working as a TUI editor(which has no "web core") plugin, or just using it in a terminal.

# Other choices
- [Golden](http://www.goldendict.org/)
- [Eudic](https://www.eudic.net/v4/en/app/eudic)
- [深蓝](https://www.ssdlsoft.com)
- ...

There are some similar products. They are all mature products, but may not suit me(or you) in some cases. Compared to these, this application is not yet so well polished, but it has its own advantages.
- It's a CLI application, conforming to the UNIX stdout convention. So you can further process the output, or embed it into any UNIX-like CLI tools.
- The output can be plain text(such as Markdown format), so feel free to render it with any renderer. You can also embed it into your extensible editor. I provide [neovim](https://neovim.io) integration out of the box.
- The output can also be internally rendered as HTML, and your can just use it to build a server, which you can access anywhere by your browser, without having to install any other apps.
- Easy to cross platform. The core is written in Go. 
- It's free and open source, just fork/PR/issue it if you like!

# Prerequisites
- Go version >=1.16, and add $GOBIN in your $PATH
- Neovim version >= 0.9.1 [recommended, because I developed it on this version, but previous versions may also use it, only some "lsp" utils (not lsp feature itself) is required. So it can also be ported to Vim, but I am not quite familiar with vim's popup feature yet]
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
A one-shot query, it will take some time when you call it the first time, it needs some loading work.
It will launch an local server using unix domain socket.

##### online engine (you don't have to specify the -e option):
```console
ondict -q <word> [-e anything]
```
![Gif](./assets/e1_online.gif)
##### mdx engine (ldoce5):
```console
ondict -q <word> -e mdx
```
![Gif](./assets/e1_mdx.gif)


#### One-shot query, but from remote server
```console
ondict -q <word> -remote localhost:1345
```
![Gif](./assets/e1_mdx_remote.gif)

#### A "repl" querier
```console
ondict -i -e mdx
```
input `.help` for commands that can be used.
![Gif](./assets/e1_mdx_interactive.gif)

#### Work as a server
This app can also serve as a HTTP server, allowing remote fetch and query, with cache and acceleration.
```console
ondict -server -listen=localhost:1345 -engine=mdx
```
Launch a http request
```console
curl "http://localhost:1345/?query=apple&engine=mdx&format=x"
```
![Gif](./assets/e1_mdx_web.gif)
If you are visiting the URL with a web browser, setting format to "html" is recommended. The browser will automatically render a more beautiful page than it is in the "CLI" interface.

You can also deploy it on your server, as an upstream of Nginx/, or just exposing it with a suitable ip/port.

You can run `make serve` locally for an easy example. My front-end skill is poor, so the page is ugly and rough, don't hate it :(. 

There are still a lot of [TODOs](./todo.md), feel free to give me PRs and contribute to the immature project, thanks in advance.

#### Work with Neovim
See [Integrated with Neovim](#neovim)
![Gif](./assets/e1_mdx_nvim.gif)

#### For MacOS, work with [hammerspoon](https://www.hammerspoon.org)
![Gif](./assets/e1_mdx_hammerspoon.gif)


## <a name="neovim"> </a>Integrated with Neovim
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
# <a name="offline"></a>Offline dictionary files
Put dictionary files in $HOME/.config/ondict/dicts, support formats are:
- "key-value" organized pairs JSON files.
- MDX files, refer to [mdict](https://mdict.org) or [pdawiki](https://pdawiki.com/forum/).

## Configuration

### file tree
```
// cd ~/.config/ondict
.
├── config.json
└── dicts
    └── Longman\ Dictionary\ of\ Contemporary\ English.mdx
```
### config.json
```json
{
    "dicts": [
        "Longman Dictionary of Contemporary English",
        "xxx",
        "yyy"
    ]
}
```
# Features
- **Online** query support based on [Longman online dictionary](https://ldoceonline.com)
- Integrated with (n)vim, feel free to use it in whatever editor you are using!
- In the offline mode, MDX engine is supported. The online engine may be more comprehensive and updated, but they are slow since an HTTP request is made for the first time. The offline mode, however, can work without internet connection, but pre-loaded [dictionary files](#offline) are needed.

# LICENSE
[LICENSE](./LICENSE)
