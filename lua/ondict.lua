local M = {}

local vimutil = require("vim.lsp.util")
-- local tutils = require ("telescope.utils")
-- local notify = tutils.notify
local notify = function(msg, _)
    vim.notify(msg, vim.log.levels.WARN)
end
-- local notify = function(funname, opts)
--   opts.once = vim.F.if_nil(opts.once, false)
--   local level = vim.log.levels[opts.level]
--   if not level then
--     error("Invalid error level", 2)
--   end
--   local notify_fn = opts.once and vim.notify_once or vim.notify
--   notify_fn(string.format("[ondict.%s]: %s", funname, opts.msg), level, {
--     title = "ondict.nvim",
--   })
-- end

local remote = "auto"
function M.setup(remote_addr)
    remote = remote_addr
end

function M.query()
    -- notify("dev version!")
    -- copy something from telescope.nvim's grep_string
    local word
    local visual = vim.fn.mode() == "v" -- TODO: v-line mode is not included
    if visual == true then
        local saved_reg = vim.fn.getreg "v"
        vim.cmd [[noautocmd sil norm "vy]]
        local selected = vim.fn.getreg "v"
        vim.fn.setreg("v", saved_reg)
        word = selected
    else
        word = vim.fn.expand "<cword>"
    end

    if vim.fn.executable "ondict" == 0 then
        notify("executable missing!", {
            msg = "ondict is not available, please refer to https://github.com/ChaosNyaruko/ondict to install it.",
            level = "ERROR"
        })
        return
    end

    -- doctor
    local output = {}
    local info = ""
    local job = { "ondict", "-q", word, "-remote", remote, "-f=md", "-e=mdx" }
    -- job = { "ondict", "-remote=auto", "-q", word, "-f=x", "-e=mdx" }
    -- notify(string.format("start query: [[ %s ]]", word))
    vim.fn.jobstart(job, {
        on_stdout = function(_, d, _)
            -- tutils.notify(string.format("on _stdout event: %s", e), {msg = string.format("ondict result, output:%s", vim.inspect(d)), level = "INFO"})
            for _, item in pairs(d) do
                table.insert(output, item)
            end
        end,
        on_stderr = function(_, _, _)
        end,
        on_exit = function(_, status, _)
            if status == 0 then
                -- notify(string.format("ondict good"), {msg = string.format("ondict result, output:%s", vim.inspect(output)), level = "INFO"})
                -- print(string.format("type output: %s, %s", type(output), vim.inspect(output)))
                output = vimutil.trim_empty_lines(output)
                info = vim.fn.join(output, "\n")
                -- notify(info)
                if info and info:len() ~= 0 then
                    notify(string.format("query [[ %s ]] from %s finished: %d", word, remote, status))
                    -- notify(vim.inspect(vim.fn.exists("w:ondict_window")))
                    if vim.fn.exists("w:ondict_window") ~= 0 and vim.api.nvim_win_get_var(0, "ondict_window") == true then
                        vim.api.nvim_win_close(0, { force = false })
                    end
                    local bufnr, winbuf, winnr =
                    -- opts.close_events = opts.close_events or { 'CursorMoved', 'CursorMovedI', 'InsertCharPre' }
                        vimutil.open_floating_preview(vimutil.convert_input_to_markdown_lines(info), "markdown",
                            {})
                    vim.api.nvim_win_set_var(winbuf, "ondict_window", true)
                    -- notify(string.format("bufnr: %s, (%s)winbuf: %s, winnr %s", bufnr, type(winbuf), winbuf, vim.inspect(winnr)))
                else
                    notify(string.format("empty valid response for [[ %s ]] ", word))
                end
            else
                -- notify("ondict error") -- TODO: ERROR doesn't always show the message, why?
            end
        end
    })
end

function M.install(path)
    local root_dir = vim.fn.expand('<sfile>:h:h')
    if path ~= "" then
        root_dir = path
    end
    if root_dir ~= "" then
        vim.cmd.lcd(root_dir)
        local res = vim.fn.system({ "go", "install", "." })
        if res == "" then
            notify(string.format("install success: <sfile>: %s, prj_dir: %s", vim.fn.expand('<sfile>'), root_dir))
            vim.cmd.lcd("-")
            return
        end
        notify(string.format("install error: %s", res))
        vim.cmd.lcd("-")
        return
    end
    notify(string.format("empty root dir, <sfile>: %s", vim.fn.expand('<sfile>')))
end

-- for quick-test
-- vim.keymap.set("n", "<leader>d", M.query)
-- M.install(".")
-- M.query()
return M
