# Android Path 3: Native APK (WebView + gomobile)

A real installable APK. Requires code changes to ondict and Android build tooling.

## Overview

```
Android APK
├── Kotlin Activity
│   └── WebView  ──────────────── loads http://127.0.0.1:1345
│
└── Go (compiled via gomobile)
    └── StartServer(configDir, cacheDir string, port int)
        └── existing Gin server — almost unchanged
```

---

## Part 1: Go Code Changes

### 1a. Abstract file paths (`util/path.go`)

Add override variables and a `SetPaths()` initializer so Android can supply its own directories:

```go
var overrideConfigPath string
var overrideTmpPath string

func SetPaths(config, tmp string) {
    overrideConfigPath = config
    overrideTmpPath = tmp
}

func ConfigPath() string {
    if overrideConfigPath != "" {
        os.MkdirAll(overrideConfigPath, 0o755)
        return overrideConfigPath
    }
    // ... existing logic ...
}

func TmpDir() string {
    if overrideTmpPath != "" {
        os.MkdirAll(overrideTmpPath, 0o755)
        return overrideTmpPath
    }
    // ... existing logic ...
}
```

### 1b. Fix SourceType gate (`render/html.go:32`)

Currently link rewriting (`entry://`, `sound://`, `@@@LINK`) only runs for Longman-type
sources. Change it to apply to all MDX sources:

```go
// Before:
if !strings.HasPrefix(h.SourceType, "LONGMAN") {
    return h.Raw
}

// After: remove the early return, always process links
```

### 1c. Create a `mobile/` package

`gomobile` cannot export a `main` package. Create a thin wrapper:

```go
// mobile/mobile.go
package mobile

import (
    "fmt"
    "net"

    "github.com/ChaosNyaruko/ondict/sources"
    "github.com/ChaosNyaruko/ondict/util"
)

// StartServer starts the Gin HTTP server on 127.0.0.1:<port>.
// configDir: app's filesDir (e.g. /data/data/<pkg>/files)
// cacheDir:  app's cacheDir (e.g. /data/data/<pkg>/cache)
func StartServer(configDir, cacheDir string, port int) {
    util.SetPaths(configDir, cacheDir)
    sources.G.Load(true, true, true) // iexact, dumpMDD, lazy
    p := NewProxy()                  // reuse server.go's NewProxy()
    l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
    if err != nil {
        return
    }
    go p.Run(l)
}
```

---

## Part 2: Build the Go `.aar`

### Prerequisites
```bash
# Go 1.23+
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init   # verifies your environment (does NOT download NDK)
```

Requires:
- Android Studio installed (for SDK; get from https://developer.android.com/studio)
- NDK installed via Android Studio: SDK Manager → SDK Tools → NDK (Side by side) (~500MB)
- `ANDROID_HOME` env var set (e.g. `~/Library/Android/sdk`)
- `ANDROID_NDK_HOME` env var set (e.g. `~/Library/Android/sdk/ndk/<version>`)

### Build
```bash
# From the ondict repo root
gomobile bind -target=android -o mobile.aar ./mobile/

# Produces:
#   mobile.aar
#   mobile-sources.jar
```

---

## Part 3: Android App (Kotlin)

### 3a. Create a new project in Android Studio
- Template: **Empty Views Activity**
- Language: Kotlin
- Min SDK: API 26 (Android 8.0)

### 3b. Add the `.aar` dependency
Copy `mobile.aar` and `mobile-sources.jar` into `app/libs/`, then in `app/build.gradle`:
```groovy
dependencies {
    implementation fileTree(dir: 'libs', include: ['*.aar', '*.jar'])
}
```

### 3c. Enable internet permission (`AndroidManifest.xml`)
```xml
<uses-permission android:name="android.permission.INTERNET"/>
```

Also add to allow cleartext localhost traffic:
```xml
<application
    android:usesCleartextTraffic="true"
    ...>
```

### 3d. Write `MainActivity.kt`
```kotlin
class MainActivity : AppCompatActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // Start the Go HTTP server in a background thread
        val configDir = filesDir.absolutePath
        val cacheDir = cacheDir.absolutePath
        Thread {
            Mobile.startServer(configDir, cacheDir, 1345)
        }.start()

        // Give the server a moment to bind
        Thread.sleep(500)

        // Load it in a WebView
        val webView = WebView(this)
        webView.settings.javaScriptEnabled = true
        webView.settings.domStorageEnabled = true
        webView.loadUrl("http://127.0.0.1:1345")
        setContentView(webView)
    }

    override fun onBackPressed() {
        val webView = (contentView as? WebView)
        if (webView?.canGoBack() == true) webView.goBack()
        else super.onBackPressed()
    }
}
```

---

## Part 4: Dictionary Files

MDX files are typically 100MB+, too large to bundle in the APK. Options:

**Option A (simplest)**: User copies files manually
- User connects phone via USB, copies `.mdx`/`.mdd` to phone storage
- App reads from `filesDir` (private) or a user-picked folder via `ACTION_OPEN_DOCUMENT_TREE`

**Option B**: Download on first launch
- App downloads dict files from a URL on first run (needs internet once)

For Option A, add a first-launch screen in Kotlin that asks the user to pick a folder
and copies files into `filesDir`.

---

## Part 5: Audio (`sound://`)

MDD audio files are dumped to `TmpDir()` by `MDict.DumpData()`. With `SetPaths()` in
place, this will write to `cacheDir` on Android. The Gin static file server already
serves from `TmpDir()`, so audio playback should work without further changes.

---

## Time Estimate

| Task | Estimated Time |
|---|---|
| Go changes (paths + mobile package + SourceType fix) | 2–3 hours |
| Android Studio + SDK + NDK setup | 2–4 hours |
| gomobile build working | 1–3 hours |
| Kotlin app + dict file loading UI | 2–4 hours |
| Debugging (permissions, audio, paths) | 1–2 days |

**Total: 1–3 days** for a working prototype assuming no prior Android experience.
