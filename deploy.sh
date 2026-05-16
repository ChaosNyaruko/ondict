#!/usr/bin/env bash
# deploy.sh — rebuild Go .aar and install APK on connected device/emulator
# Usage: ./deploy.sh [--force]   (--force re-runs gomobileBind even if unchanged)

set -e

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
ANDROID_DIR="$REPO_DIR/android"
ADB="${ADB:-$HOME/Library/Android/sdk/platform-tools/adb}"

# Use Android Studio's bundled JDK if JAVA_HOME isn't already set.
AS_JDK="/Applications/Android Studio.app/Contents/jbr/Contents/Home"
if [ -z "$JAVA_HOME" ] && [ -d "$AS_JDK" ]; then
    export JAVA_HOME="$AS_JDK"
    export PATH="$JAVA_HOME/bin:$PATH"
fi

export ANDROID_HOME="${ANDROID_HOME:-$HOME/Library/Android/sdk}"

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

# ---- 3. Gradle build + install ----
echo "==> Building and installing APK..."
cd "$ANDROID_DIR"
if [ "$1" = "--force" ]; then
    ./gradlew :app:gomobileBind --rerun-tasks
else
    ./gradlew :app:gomobileBind
fi
./gradlew installDebug

# ---- 4. Launch the app ----
echo "==> Launching app..."
"$ADB" shell am start -n com.ondict.app/.MainActivity

echo ""
echo "Done."
