local M = {}

local notify = function(msg, _)
    vim.notify(msg, vim.log.levels.WARN)
end

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
    local job = { "ondict", "-q", word, "-remote", remote, "-f", "md", "-e", "mdx" }
    -- local job = { "ondict", "-version" }
    -- job = { "ondict", "-remote=auto", "-q", word, "-f=x", "-e=mdx" }
    -- notify(string.format("start query: [[ %s ]]", word))
    vim.fn.jobstart(job, {
        on_stdout = function(_, d, _)
            -- tutils.notify(string.format("on _stdout event: %s", e), {msg = string.format("ondict result, output:%s", vim.inspect(d)), level = "INFO"})
            for _, item in pairs(d) do
                -- print(item)
                table.insert(output, item)
            end
        end,
        on_stderr = function(_, _, _)
        end,
        on_exit = function(_, status, _)
            if status == 0 then
                -- notify(string.format("ondict good"), {msg = string.format("ondict result, output:%s", vim.inspect(output)), level = "INFO"})
                output = vim.tbl_filter(function(line)
                    return line ~= ""
                end, output)
                -- print(string.format("type output: %s, %s", type(output), vim.inspect(output)))
                info = vim.fn.join(output, "\n")
                -- notify(info)
                if info and info:len() ~= 0 then
                    notify(string.format("query [[ %s ]] from %s finished: %d", word, remote, status))
                    M.show(info)
                else
                    notify(string.format("empty valid response for [[ %s ]] ", word))
                end
            else
                -- notify("ondict error")
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

-- Reusable state
local state = {
    buf = nil, -- scratch buffer handle
    win = nil, -- floating window handle
}

-- Helper: ensure we have a scratch buffer to reuse
local function ensure_buf()
    if state.buf and vim.api.nvim_buf_is_valid(state.buf) then
        return state.buf
    end
    local buf = vim.api.nvim_create_buf(false, true) -- [listed=false, scratch=true]
    -- Make it not modifiable by user edits, but we'll toggle when setting lines
    vim.api.nvim_set_option_value("buftype", "nofile", { buf = buf })
    vim.api.nvim_set_option_value("swapfile", false, { buf = buf })
    vim.api.nvim_set_option_value("bufhidden", "hide", { buf = buf })
    vim.api.nvim_set_option_value("modifiable", false, { buf = buf })
    vim.api.nvim_set_option_value("filetype", "markdown", { buf = buf }) -- optional
    state.buf = buf
    return buf
end

-- Helper: open or reuse a floating window for the buffer
local function open_float(buf, opts)
    opts           = opts or {}

    -- Determine size and position
    local ui       = vim.api.nvim_list_uis()[1]
    local max_w    = ui and ui.width or 120
    local max_h    = ui and ui.height or 40

    local width    = math.min(opts.width or 60, max_w - 2)
    local height   = math.min(opts.height or 10, max_h - 2)

    local row      = math.floor((max_h - height) / 2)
    local col      = math.floor((max_w - width) / 2)

    local win_opts = {
        relative = "editor",
        row = row,
        col = col,
        width = width,
        height = height,
        anchor = "NW",
        style = "minimal",
        border = opts.border or "rounded",
        noautocmd = true,
    }

    if state.win and vim.api.nvim_win_is_valid(state.win) then
        -- Reuse existing window; just set the buffer
        vim.api.nvim_win_set_buf(state.win, buf)
    else
        state.win = vim.api.nvim_open_win(buf, true, win_opts)
        -- Window-local options
        vim.api.nvim_set_option_value("number", false, { win = state.win })
        vim.api.nvim_set_option_value("relativenumber", false, { win = state.win })
        vim.api.nvim_set_option_value("wrap", true, { win = state.win })
        vim.api.nvim_set_option_value("cursorline", false, { win = state.win })
    end

    return state.win
end

-- Public: show a list of strings in the floating buffer/window
-- lines: string[] | string
-- opts: { width?, height?, border? }
function M.show(lines, opts)
    local buf = ensure_buf()

    -- Normalize input to a list of lines
    if type(lines) == "string" then
        lines = vim.split(lines, "\n", { plain = true })
    elseif type(lines) ~= "table" then
        lines = { vim.inspect(lines) }
    end

    -- Update buffer content
    vim.api.nvim_set_option_value("modifiable", true, { buf = buf })
    vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
    vim.api.nvim_set_option_value("modifiable", false, { buf = buf })

    -- Open or reuse floating window
    open_float(buf, opts)
end

-- Optional: close the window but keep buffer for reuse
function M.hide()
    if state.win and vim.api.nvim_win_is_valid(state.win) then
        vim.api.nvim_win_close(state.win, true)
        state.win = nil
    end
end

-- Optional: completely dispose state (buffer + window)
function M.dispose()
    if state.win and vim.api.nvim_win_is_valid(state.win) then
        vim.api.nvim_win_close(state.win, true)
        state.win = nil
    end
    if state.buf and vim.api.nvim_buf_is_valid(state.buf) then
        vim.api.nvim_buf_delete(state.buf, { force = true })
        state.buf = nil
    end
end

-- for quick-test
-- vim.keymap.set("n", "<leader>d", M.query)
-- M.install(".")
-- M.query()
return M
