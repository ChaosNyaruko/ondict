# history/auto.go Review: SQL query built with fmt.Sprintf (not parameterized)

**Severity:** LOW
**File:** `history/auto.go:92-99`

**Description:**
The `Review` function builds the WHERE clause with `fmt.Sprintf`:

```go
where := fmt.Sprintf(`WHERE 
    update_time > datetime('now', 'localtime', '-%d days')
    AND count >= %d `, d, cnt)
rows, err := db.Query(`SELECT * FROM history ` + where + `ORDER BY ...`)
```

`d` and `cnt` are already validated as integers via `strconv.Atoi`, so there is no
immediate SQL-injection risk. However, the pattern of string-formatting into SQL is
fragile and could be copied to locations where untrusted strings are used.

**Advice:**
Use SQLite's datetime modifier with a bound parameter instead:

```go
rows, err := db.Query(
    `SELECT * FROM history WHERE update_time > datetime('now', 'localtime', ? || ' days') AND count >= ? ORDER BY update_time DESC`,
    fmt.Sprintf("-%d", d), cnt,
)
```
