#!/usr/bin/env bash
# android-observe.sh
#
# Observe ondict Android app launch: CPU usage over time + Go timing logs.
#
# Usage:
#   ./scripts/android-observe.sh [duration_seconds]
#
# Examples:
#   ./scripts/android-observe.sh        # default 120s (enough for first-launch dump)
#   ./scripts/android-observe.sh 30     # quick check for SQLite launch
#
# Requirements:
#   - Android device connected via USB with USB Debugging enabled
#   - adb at ~/Library/Android/sdk/platform-tools/adb (or on PATH)

set -euo pipefail

ADB="${ADB:-$(command -v adb 2>/dev/null || echo "$HOME/Library/Android/sdk/platform-tools/adb")}"
PACKAGE="com.ondict.app"
ACTIVITY="${PACKAGE}/.MainActivity"
DURATION="${1:-120}"
CPU_LOG="/tmp/ondict_cpu.txt"

if ! "$ADB" devices | grep -q "device$"; then
  echo "error: no Android device found. Check USB connection and USB Debugging."
  exit 1
fi

echo "==> force-stopping $PACKAGE"
"$ADB" shell am force-stop "$PACKAGE"

echo "==> clearing logcat buffer"
"$ADB" logcat -c

echo "==> launching $ACTIVITY"
"$ADB" shell am start -n "$ACTIVITY"

echo "==> recording CPU on-device for ${DURATION}s (output: $CPU_LOG)"
# Run top entirely on-device to avoid USB round-trip jitter per sample.
# -d 1  : sample every 1 second
# -q    : suppress header/summary lines, process rows only
# toybox top does not support -b (Linux batch mode) — omit it.
"$ADB" shell "top -d 1 -q | grep --line-buffered $PACKAGE" > "$CPU_LOG" &
TOP_PID=$!

echo "    (press Ctrl-C to stop early)"
sleep "$DURATION" && kill "$TOP_PID" 2>/dev/null || true

echo ""
echo "=== TIMING LOGS (from logcat) ========================="
"$ADB" logcat -d | grep "GoLog" | grep -E "\[timing\]|vocab\.db|StartServer|auto-dump|stuck at" || echo "(none found — buffer may have rolled over on a long dump)"

echo ""
echo "=== CPU SAMPLES (from $CPU_LOG) ======================="
# Print a condensed view: timestamp (col 1 from top) + %CPU field
# toybox top row format: PID USER PR NI VIRT RES SHR S %CPU %MEM TIME ARGS
awk '{printf "%-6s  CPU: %s%%  MEM: %s%%  RES: %s\n", NR"s", $9, $10, $6}' "$CPU_LOG"
