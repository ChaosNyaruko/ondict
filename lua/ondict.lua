local M = {}

local tutils = require ("telescope.utils")
local vimutil = require("vim.lsp.util")

function M.query()
    -- copy something from telescope.nvim's grep_string
    local word
    local visual = vim.fn.mode() == "v"
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
        tutils.notify("executable missing!", {
            msg = "ondict is not available, please refer to https://github.com/ChaosNyaruko/ondict to install it.", level = "ERROR"})
        return
    end

    -- doctor
    local output = {}
    local info = ""
    vim.fn.jobstart({"ondict", "-q", word}, {
        on_stdout = function(_, d, _)
            -- tutils.notify(string.format("on _stdout event: %s", e), {msg = string.format("ondict result, output:%s", vim.inspect(d)), level = "INFO"})
            for _, item in pairs(d) do
                table.insert(output, item)
            end
        end,
        on_exit = function(_, status, _)
            -- tutils.notify(string.format("exit event: %s", event), {msg = string.format("ondict result, output:%s", vim.inspect(output)), level = "INFO"})
            if status == 0 then
                -- tutils.notify(string.format("ondict good"), {msg = string.format("ondict result, output:%s", vim.inspect(output)), level = "INFO"})
                -- print(string.format("type output: %s, %s", type(output), vim.inspect(output)))
                output = vimutil.trim_empty_lines(output)
                info = vim.fn.join(output, "\n")
            else
                info = "ondict error"
                -- tutils.notify(string.format("ondict error"), {msg = string.format("ondict result, output:%s", vim.inspect(status)), level = "WARN"})
            end
            -- print(string.format("type info: %s, %s", type(info), info))
            vimutil.open_floating_preview(vimutil.convert_input_to_markdown_lines(info), "markdown", {})
        end
    })
    end

    ----- Telescope Wrapper around vim.notify
    -----@param funname string: name of the function that will be
    -----@param opts table: opts.level string, opts.msg string, opts.once bool
    --utils.notify = function(funname, opts)
    --  opts.once = vim.F.if_nil(opts.once, false)
    --  local level = vim.log.levels[opts.level]
    --  if not level then
    --    error("Invalid error level", 2)
    --  end
    --  local notify_fn = opts.once and vim.notify_once or vim.notify
    --  notify_fn(string.format("[telescope.%s]: %s", funname, opts.msg), level, {
    --    title = "telescope.nvim",
    --  })
    --end
    --

-- for quick-test
-- vim.keymap.set("n", "<leader>d", M.query)
return M

