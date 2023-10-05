local M = {}

local vimutil = require("vim.lsp.util")
-- local tutils = require ("telescope.utils")
-- local notify = tutils.notify
local notify = function(funname, _)
    vim.notify(funname, vim.log.levels.WARN)
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


function M.query()
    -- notify("??")
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
        notify("executable missing!", {
            msg = "ondict is not available, please refer to https://github.com/ChaosNyaruko/ondict to install it.", level = "ERROR"})
        return
    end

    -- doctor
    local output = {}
    local info = ""
    vim.fn.jobstart({"ondict", "-q", word, "-c", "auto"}, {
        on_stdout = function(_, d, _)
            -- tutils.notify(string.format("on _stdout event: %s", e), {msg = string.format("ondict result, output:%s", vim.inspect(d)), level = "INFO"})
            for _, item in pairs(d) do
                table.insert(output, item)
            end
        end,
        on_stderr = function(_, d, _)
            notify(vim.inspect(d))
        end,
        on_exit = function(_, status, _)
            if status == 0 then
                -- notify(string.format("ondict good"), {msg = string.format("ondict result, output:%s", vim.inspect(output)), level = "INFO"})
                -- print(string.format("type output: %s, %s", type(output), vim.inspect(output)))
                output = vimutil.trim_empty_lines(output)
                info = vim.fn.join(output, "\n")
                vimutil.open_floating_preview(vimutil.convert_input_to_markdown_lines(info), "markdown", {})
            else
                -- notify("ondict error") -- TODO: ERROR doesn't always show the message, why?
            end
        end
    })
    end

-- for quick-test
-- vim.keymap.set("n", "<leader>d", M.query)
return M

