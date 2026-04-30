# decoder/mdx.go DumpKeys: progress bar writes to stdout, corrupting CLI pipe output

**Severity:** LOW
**File:** `decoder/mdx.go:98`

**Description:**
`progressbar.Default(...)` writes directly to `os.Stdout`. When `ondict` is used as
a Unix filter (e.g., piped to another command), this progress bar output corrupts the
actual dictionary result that the downstream program reads. The same issue occurs in
`DumpData` (line 660) and `DumpDict` (line 336).

**Advice:**
Write progress bars to `os.Stderr` so stdout remains clean:

```go
bar := progressbar.NewOptions64(int64(m.numEntries),
    progressbar.OptionSetWriter(os.Stderr),
    progressbar.OptionSetDescription(...),
)
```
