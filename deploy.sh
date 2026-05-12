#!/usr/bin/env bash
# deploy.sh — rebuild Go .aar and install APK on connected device/emulator
# Usage: ./deploy.sh [--force]   (--force re-runs gomobileBind even if unchanged)

set -e

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
ANDROID_DIR="$REPO_DIR/android"
ADB="$HOME/Library/Android/sdk/platform-tools/adb"

# ---- 1. Check emulator/device is connected ----
echo "==> Checking device..."
if ! "$ADB" devices | grep -q "device$"; then
    echo "No device/emulator connected. Start one first."
    exit 1
fi

# ---- 2. Run Go tests ----
echo "==> Running Go tests..."
cd "$REPO_DIR"
go test ./...

# ---- 3. Gradle build + install (gomobileBind runs automatically if needed) ----
echo "==> Building and installing APK..."
cd "$ANDROID_DIR"
if [ "$1" = "--force" ]; then
    ./gradlew :app:gomobileBind --rerun-tasks installDebug
else
    ./gradlew installDebug
fi

# ---- 4. Launch the app ----
echo "==> Launching app..."
"$ADB" shell am start -n com.ondict.app/.MainActivity

echo ""
echo "Done."
