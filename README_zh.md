目录
=================

* [简介](#简介)
* [免责声明](#免责声明)
* [其他选择](#其他选择)
* [前置要求](#前置要求)
* [功能特性](#功能特性)
* [安装](#安装)
* [使用方法](#使用方法)
   * [帮助](#帮助)
   * [使用示例](#使用示例)
      * [作为HTTP服务器运行](#作为http服务器运行)
      * [单次查询](#单次查询)
      * [从远程服务器进行单次查询](#从远程服务器进行单次查询)
      * [交互式查询](#交互式查询)
      * [与Neovim集成](#与neovim集成)
      * [与MacOS的hammerspoon集成](#与macos的hammerspoon集成)
      * [与FZF集成（实验性功能，仅支持MacOS）](#与fzf集成实验性功能仅支持macos)
* [如何在Neovim中使用](#neovim)
* [配置](#配置)
* [待办事项](#待办事项)
* [许可证](#许可证)
* [目录](#目录)

<!-- 由 https://github.com/ekalinin/github-markdown-toc 创建 -->
-----

# 简介
这是另一个简单的词典应用。支持多种数据源，包括朗文在线词典和用户加载的MDX/MDD词典文件。

[视频介绍（简中）](https://www.bilibili.com/video/BV1PuB5YmEeq)

# 免责声明
本应用旨在作为一个**词典**工具，可在您学习英语或撰写英文文章时使用。

它**不是**翻译器，对于翻译需求，现代基于LLM模型的工具更为合适。

推荐使用Web模式，因为在线引擎和MDX引擎都基于HTML/CSS。如果需要将输出渲染为markdown格式，则需要为每个数据源单独编写解析器和渲染器，这工作量很大且几乎不可能完成。

我只为[朗文当代英语词典](https://github.com/ChaosNyaruko/ondict/releases/download/v0.0.5/Longman.Dictionary.of.Contemporary.English.mdx)编写了一个简单的markdown渲染器，您可以在某些发布版本中下载到，这样您就可以在某些情况下（比如在没有"web核心"的TUI编辑器插件中，或者直接在终端中）粗略地使用其markdown渲染输出。

# 其他选择
- [ninja33的出色作品](https://github.com/ninja33/mdx-server)
- [Golden](http://www.goldendict.org/)
- [欧路词典](https://www.eudic.net/v4/en/app/eudic)
- [深蓝词典](https://www.ssdlsoft.com)
- ...

市面上有一些类似的产品。它们都是成熟的产品，但可能不太适合我（或您）的某些使用场景。相比之下，这个应用虽然还不够完善，但也有其独特的优势：
- 可以作为CLI应用程序使用，符合UNIX标准输出约定。因此您可以进一步处理输出，或将其嵌入到任何类UNIX的CLI工具中，然后轻松地集成到您的CLI/TUI工作流程中。
- 输出可以是纯文本（如Markdown格式），因此您可以使用任何渲染器来渲染它。您还可以将其嵌入到您的可扩展编辑器中。我们提供了开箱即用的[neovim](https://neovim.io)集成。
- 原始输出（特别是MDX词典）就是HTML/CSS/JavaScript，所以您可以直接用它搭建服务器，通过浏览器随时随地访问，无需安装任何其他应用。例如：[演示网站](https://mini.freecloud.dev)
- 易于跨平台。核心使用Go语言编写。
- 免费开源，如果您喜欢就fork/PR/issue吧！

# 前置要求
- Go版本 >=1.16，并将$GOBIN添加到您的$PATH中
- Neovim版本 >= 0.9.1（推荐，因为我是在这个版本上开发的，但之前的版本也可能可以使用，只需要一些"lsp"工具（不是lsp功能本身）。理论上也可以稍作修改移植到Vim上，但我目前还不太熟悉vim的popup功能）

# 功能特性
- 可以Web模式部署在私有服务器上，没有任何遥测
- 基于[朗文在线词典](https://ldoceonline.com)的**在线**查询支持
- 与(n)vim集成，随意在您使用的任何编辑器中使用！
- 在离线模式下，支持MDX引擎。在线引擎可能更全面和更新，但由于首次需要进行HTTP请求，所以速度较慢。而离线模式可以在没有网络连接的情况下工作，但需要预先加载[词典文件](#离线)。

# 安装
## 从源码构建（推荐）
```console
go install github.com/ChaosNyaruko/ondict@latest
```
或
```console
git clone https://github.com/ChaosNyaruko/ondict.git
make install
```
## 使用Docker并在容器中作为HTTP服务器运行
为了方便起见，推荐将容器中的配置目录被重新映射/挂载到您的主机配置目录，所有生成的内容（如查询历史）都会被转储到这个目录中。不会产生除此以外其他对主机文件系统的污染。
### 本地
```console
docker build . -t ondict
docker run --rm --name ondict-app --publish 1345:1345 --mount type=bind,source={your $HOME/.config/ondict},target=/root/.config/ondict  ondict
```
### 远程
```console
docker run --rm --name ondict-app --publish 1345:1345 --mount type=bind,source={your $HOME/.config/ondict},target=/root/.config/ondict  chaosnyaruko/ondict:latest
```

# 可单独使用的解析工具
```console
go install github.com/ChaosNyaruko/ondict/cmd/dumpdict@latest
```
如果你并不需要一个完整的server或词典工具，只是想解析一下MDX文件拿到里面的内容，你可以使用上述命令安装一个dumpdict工具。

这个工具主要功能是解析MDX文件，并把它们记录到一个sqlite3数据库的文件中。

可以参考[这个文件](./schema.sql)中的vocab表结构进行查看，或二次开发！

# 使用方法
## 帮助
```console
ondict 
```
```console
ondict -h
```

## 使用示例
### 作为HTTP服务器运行
又称Web模式，**推荐使用**。

本应用可以作为HTTP服务器运行，支持远程获取和查询，具有缓存和加速功能。
```console
ondict -serve -listen=localhost:1345 -e=mdx
```
发起HTTP请求
```console
curl "http://localhost:1345/?query=apple&engine=mdx&format=x"
```
或者直接打开浏览器，访问localhost:1345即可！

如果您使用网页浏览器访问URL，建议将format设置为"html"。浏览器将自动渲染出比"CLI"界面更美观的页面。

您也可以将其部署在您的服务器上，作为Nginx的上游，或者直接用合适的ip/端口暴露它。

您可以在本地运行`make serve`来查看简单示例。由于我的前端技能有限，页面可能比较简陋，请见谅 :(。

还有很多[待办事项](./todo.md)，欢迎给我提PR，为这个尚未成熟的项目做出贡献，提前感谢。

### 单次查询
单次查询会在首次调用时需要一些时间，因为需要进行一些加载工作。
它会使用unix domain socket启动一个本地服务器。

#### 在线引擎（不需要指定-e选项）：
```console
ondict -q <word> [-e anything]
```
![Gif](./assets/e1_online.gif)

#### mdx引擎（ldoce5）：
```console
ondict -q <word> -e mdx
```
![Gif](./assets/e1_mdx.gif)

### 从远程服务器进行单次查询
```console
ondict -q <word> -remote localhost:1345
```
![Gif](./assets/e1_mdx_remote.gif)

### 交互式查询
```console
ondict -i -e mdx
```
输入 `.help` 查看可用命令。
![Gif](./assets/e1_mdx_interactive.gif)

### 与Neovim集成
参见[与Neovim集成](#neovim)
![Gif](./assets/e1_mdx_nvim.gif)

# <a name="neovim"></a>如何在Neovim中使用
1. 使用插件管理器或手动安装插件。
2. 使用 `:lua require("ondict").query()` 来查询光标下的单词（\<cword\>）。
3. 为自己定义一个更方便的映射来调用它。注意：在可视模式下，请使用 "\<cmd\>lua require("ondict").query()\<cr\>"。这样可以捕获"选中"的单词。否则，"模式"会被改变，只能查询光标下的单词（\<cword\>）。

## 使用[lazy插件管理器](https://github.com/folke/lazy.nvim)，比较推荐这一种（现在应该很少neovim用户手动装插件吧）
```lua
require("lazy").setup({
  spec = {
    -- add your plugins here
    {
        "ChaosNyaruko/ondict",
        event = "VeryLazy",
        build = function(plugin)
            require("ondict").install(plugin.dir)
        end,
        dev = false,
        config = function()
            require("ondict").setup("localhost:1345") -- If you already have a running ondict server, you can just specify the address.
        end
    },
  },
  -- Configure any other settings here. See the documentation for more details.
  -- colorscheme that will be used when installing plugins.
  -- automatically check for plugin updates
  checker= { enabled = false },
})
```

## 手动安装：
```console
cd ~/.local/share/nvim/site/pack/packer/start/
git clone https://github.com/ChaosNyaruko/ondict.git
cd ondict
go install .
```

### 映射示例
```vimscript
nnoremap <leader>d <cmd>lua require("ondict").query()<cr>
vnoremap <leader>d <cmd>lua require("ondict").query()<cr>
```

```lua
vim.keymap.set("n", "<leader>d", require("ondict").query)
vim.keymap.set("v", "<leader>d", require("ondict").query)
```

### 与MacOS的hammerspoon集成
![Gif](./assets/e1_mdx_hammerspoon.gif)

##### 已知问题：
如果您使用hammerspoon的"task"功能（即"hs.task.new"然后"xx::start"），某些词的查询会阻塞进程，无法看到结果（因为还没有返回），比如"test"。但在真正的web模式下没有这样的问题，这种情况只在hammerspoon中出现。

目前还不知道原因，同样的词查询在[Neovim集成](#neovim)中也能正常工作，后者也使用Lua作为其异步运行时。因此我猜测可能与实现有关，这可能是hammerspoon的一个bug。

##### 解决方案
使用hs.execute代替hs.task（注意shell转义），这是执行任务的"同步"方法。普通查询足够快，您不会注意到差异，会"立即"看到结果。参见[示例](https://github.com/ChaosNyaruko/dotfiles/blob/mini/hammerspoon/init.lua#L90)

### 与FZF集成（实验性功能，仅支持MacOS）
```console
ondict -fzf
```
您需要安装[FZF](https://github.com/junegunn/fzf)，并且ondict服务器监听在localhost:1345（目前正在开发中）
![Gif](./assets/ondict_fzf.gif)

# <a name="离线"></a>离线词典文件
将词典文件放在$HOME/.config/ondict/dicts中，支持的格式有：
- "键值对"组织的JSON文件。
- MDX文件，参考[mdict](https://mdict.org)或[pdawiki](https://pdawiki.com/forum/)。

# 配置

## XDG_CONFIG_HOME约定
```
// cd ~/.config/ondict
.
├── config.json
├── dicts
│   ├── LDOCE5++ V 1-35.mdd
│   ├── LDOCE5++ V 1-35.mdx
│   ├── LM5style.css
│   ├── LM5style_vanilla.css
│   ├── Longman Dictionary of Contemporary English.css
│   ├── Longman Dictionary of Contemporary English.mdx
│   ├── ODE_Zh.css
│   ├── ahd3af.css
│   ├── oald9.css
│   ├── oald9.mddx
│   └── oald9.mdx
└── history.table
```
## config.json示例
```json
{
  "dicts": [
    {
      "name": "LDOCE5++ V 1-35",
      "type": "LONGMAN5/Online"
    },
    {
      "name": "Longman Dictionary of Contemporary English",
      "type": "LONGMAN/Easy"
    },
    {
      "name": "oald9"
    }
  ]
}
```

# 待办事项
还有很多[待办事项](./todo.md)，欢迎给我提PR，为这个尚未成熟的项目做出贡献，提前感谢。

# 许可证
[许可证](./LICENSE) 


