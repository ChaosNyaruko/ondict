#!/usr/bin/env bash
# release.sh — build a signed release APK and optionally create a GitHub release
# Usage:
#   ./release.sh                        # build APK, print output path
#   ./release.sh --publish v1.0.0       # build APK + create GitHub release with tag

set -e

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
ANDROID_DIR="$REPO_DIR/android"
APK_PATH="$ANDROID_DIR/app/build/outputs/apk/release/app-release.apk"

# Use Android Studio's bundled JDK if JAVA_HOME isn't already set.
AS_JDK="/Applications/Android Studio.app/Contents/jbr/Contents/Home"
if [ -z "$JAVA_HOME" ] && [ -d "$AS_JDK" ]; then
    export JAVA_HOME="$AS_JDK"
    export PATH="$JAVA_HOME/bin:$PATH"
fi

export ANDROID_HOME="${ANDROID_HOME:-$HOME/Library/Android/sdk}"

# ---- Signing credentials (override via env vars in CI) ----
export KEYSTORE_PATH="${KEYSTORE_PATH:-$REPO_DIR/ondict-release.jks}"
export KEYSTORE_PASSWORD="${KEYSTORE_PASSWORD:-ondictpass}"
export KEY_ALIAS="${KEY_ALIAS:-ondict}"
export KEY_PASSWORD="${KEY_PASSWORD:-ondictpass}"

if [ ! -f "$KEYSTORE_PATH" ]; then
    echo "Keystore not found at $KEYSTORE_PATH"
    echo "Run keytool to generate it first (see release docs)."
    exit 1
fi

# ---- 1. Go tests ----
echo "==> Running Go tests..."
cd "$REPO_DIR"
go test ./...

# ---- 2. Build signed release APK ----
echo "==> Building signed release APK..."
cd "$ANDROID_DIR"
./gradlew :app:gomobileBind
./gradlew assembleRelease

echo ""
echo "APK: $APK_PATH"
echo "Size: $(du -sh "$APK_PATH" | cut -f1)"

# ---- 3. Optionally publish to GitHub Releases ----
if [ "$1" = "--publish" ]; then
    TAG="${2:-}"
    if [ -z "$TAG" ]; then
        echo "Usage: ./release.sh --publish <tag>   e.g. ./release.sh --publish v1.0.0"
        exit 1
    fi

    if ! command -v gh &>/dev/null; then
        echo "'gh' CLI not found. Install it from https://cli.github.com/ to publish releases."
        exit 1
    fi

    echo ""
    echo "==> Creating GitHub release $TAG..."
    cd "$REPO_DIR"
    git tag "$TAG"
    git push origin "$TAG"
    gh release create "$TAG" "$APK_PATH" \
        --title "Ondict $TAG" \
        --notes "Release $TAG"
    echo "Published: $(gh release view "$TAG" --json url -q .url)"
fi
