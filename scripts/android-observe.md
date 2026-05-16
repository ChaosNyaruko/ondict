# android-observe.sh

A shell script for observing the ondict Android app's launch cost: CPU usage
over time and Go server timing logs, captured via adb.

## Why this exists

The ondict Android app embeds a Go HTTP server (via gomobile) that decodes MDX
dictionary files on startup. This is expensive on first launch. We added a
background SQLite dump so subsequent launches skip MDX decoding entirely and
load from `vocab.db` instead. This script lets you measure and compare the two
cases side by side.

## Usage

```bash
./scripts/android-observe.sh [duration_seconds]
```

- Default duration is 120s — enough for a normal SQLite launch with room to spare
- Use 180s+ for a first launch with a large MDX file, where the background dump
  can take 2-3 minutes
- Use 30s for a quick SQLite launch check

## What it does, step by step

1. **Force-stops the app** (`adb shell am force-stop`) so we always measure a
   cold start, not a resume from background
2. **Clears the logcat ring buffer** (`adb logcat -c`) so timing logs from a
   previous run don't pollute the results
3. **Launches the app** (`adb shell am start -n`)
4. **Records CPU on-device** using `adb shell top -d 1 -q | grep ondict`,
   piped to `/tmp/ondict_cpu.txt` on the Mac. Running `top` on the device
   rather than polling from the Mac avoids USB round-trip jitter (~100-200ms
   per sample)
5. **Pulls timing logs** from logcat after the observation window, filtering
   for the `[timing]` markers emitted by the Go server
6. **Formats CPU output** with `awk` into a readable per-second table

## How CPU measurement works

Android's `top` reads from `/proc/<pid>/stat`, updated by the kernel every
scheduler tick. The `%CPU` column is:

```
CPU time consumed in the last sample interval
───────────────────────────────────────────── × 100
wall-clock length of the sample interval
```

Values above 100% are normal — they mean the process is using more than one
core. The phone used during development has multiple cores available to apps,
so ~100% means one full core saturated, ~150% means one and a half cores.

### Why not `-b` (batch mode)?

`-b` is a Linux `top` flag. Android uses `toybox top`, which does not support
it. We omit it here. Some toybox versions silently ignore unknown flags (which
is why earlier ad-hoc commands appeared to work), but relying on that is
fragile.

The correct toybox top flags used here:
- `-d 1` — sample every 1 second
- `-q` — suppress the header and system summary lines, output process rows only

## How timing logs work

The Go server writes structured logs via `logrus`. In `mobile/mobile.go`,
`logrus` output is directed to both a file (`<cacheDir>/ondict.log`) and
`os.Stderr`. Android captures anything written to stderr and tags it in logcat
as `GoLog`. The key log lines emitted are:

| Log message | Where emitted | What it means |
|---|---|---|
| `StartServer: configDir=... port=...` | `mobile.go` | Go server goroutine started |
| `[timing] vocab.db loaded in Xms` | `sources/mdx.go` | SQLite path taken, MDX skipped |
| `[timing] MDX Register took Xms` | `sources/mdx.go` | MDX path taken, key index built |
| `[timing] G.Load took Xms` | `mobile.go` | Total dict load time |
| `[timing] server ready in Xms` | `mobile.go` | Server is listening, app is usable |
| `auto-dump: starting background dump` | `sources/mdx.go` | Background SQLite dump started |
| `auto-dump: vocab.db ready in Xm Xs` | `sources/mdx.go` | Dump complete, next launch faster |

## Why logcat can miss lines

The logcat ring buffer is finite (~256KB by default). On a first launch with a
large dictionary, the background dump produces enough log output to flush the
buffer, which means early `[timing]` lines get evicted before you pull them.
Workaround: reduce log level to `InfoLevel` and remove debug-level lines in hot
paths, or increase the buffer size with `adb logcat -G 8M`.

## Interpreting results

### First launch (MDX path, no vocab.db yet)

```
1s   CPU: 144%   — Go runtime + JVM init
2s   CPU: 96%    — MDX key index decode starts
3–84s CPU: ~100%  — background SQLite dump running
85s  CPU: 25%    — dump finishing, FTS index build winding down
```

The app is **already usable** for queries from ~t+1s. The CPU cost is from the
background dump goroutine, invisible to the user.

### Second launch (SQLite path, vocab.db complete)

```
1s   CPU: 96%    — Go runtime + JVM init only
2s+  CPU: 0%     — idle, server already listening
```

```
[timing] vocab.db loaded in 89ms — skipping MDX decode
[timing] G.Load took 89ms
[timing] server ready in 89ms
```

89ms vs 85+ seconds of sustained CPU. The SQLite path is essentially free.

## Requirements

- macOS with adb at `~/Library/Android/sdk/platform-tools/adb` or on `$PATH`
  (override with `ADB=/path/to/adb ./scripts/android-observe.sh`)
- Android device connected via USB with USB Debugging enabled
- The ondict app installed on the device
