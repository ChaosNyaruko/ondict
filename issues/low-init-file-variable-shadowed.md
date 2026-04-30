# main.go init(): package-level `file` variable never assigned (shadowed by :=)

**Severity:** LOW
**File:** `main.go:69-80`

**Description:**
The package-level `var file *os.File` is intended to hold the log file descriptor,
but the `init()` function uses `:=` instead of `=`:

```go
var file *os.File   // package level

func init() {
    file, err := os.OpenFile(...)   // := shadows the package-level file
    if err == nil {
        log.SetOutput(file)   // uses the local variable, fine
    }
    // package-level file is still nil
}
```

The package-level `file` is never assigned and is never used anywhere else in the
codebase, so this is a resource-management dead end — the file descriptor is opened
but there is no way to close it later.

**Advice:**
Either assign to the package-level variable properly (`file, err = os.OpenFile(...)`)
so it can be closed at shutdown, or (simpler) remove the package-level `var file` and
make `file` entirely local to `init()`. The log file will still be used by logrus
because `log.SetOutput` holds a reference.
