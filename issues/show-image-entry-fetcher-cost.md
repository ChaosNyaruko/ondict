# ShowImageHandler — EntryFetcher Cost

## Summary

When rendering an entry that contains `<a class="ldoce-show-image" base64="ldoce4188jpg">`
(e.g. "apple"), `ShowImageHandler.resolveViaEntryFetcher` calls
`QueryMDX("fruit", "html_fragment")` to extract the `big_pic` image path.

The call chain is:
1. Render "apple" → `ShowImageHandler` fires
2. `ctx.EntryFetcher("fruit")` → `QueryMDX("fruit", "html_fragment")`
3. Full render of "fruit" entry (DOM parse + all handlers + serialisation)
4. `bigPicSrc` re-parses the rendered HTML string to read one `<img src>`

## Why it is deferred

- Only fires for entries with an `ldoce-show-image` cross-ref to another entry.
  These are rare (illustrated words like "apple", "car", "house").
- Not a loop — bounded to one extra render call per such element.
- The cost is acceptable for now.

## Potential optimisation

`big_pic` `<img src>` is present verbatim in the raw MDX HTML — it does not require
rendering to extract. A cheaper path would be:

1. Add a `RawFetcher func(word string) string` to `RenderContext` that returns the
   raw (unrendered) definition string.
2. `bigPicSrc` reads the raw string directly (one HTML parse, no render).
3. Remove the rendered `html_fragment` path for this use case entirely.

This avoids one full render + one extra HTML re-parse. Implement if profiling shows
this is a bottleneck (e.g. on mobile where "apple" is a common lookup).
