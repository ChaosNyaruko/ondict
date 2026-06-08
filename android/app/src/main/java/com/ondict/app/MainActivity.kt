package com.ondict.app

import android.content.Intent
import android.media.AudioAttributes
import android.media.MediaPlayer
import android.os.Bundle
import android.view.KeyEvent
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.InputMethodManager
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.ArrayAdapter
import android.widget.Button
import android.widget.EditText
import android.widget.ListView
import androidx.appcompat.app.AppCompatActivity
import mobile.Mobile
import java.io.ByteArrayInputStream
import java.net.HttpURLConnection
import java.net.URL

class MainActivity : AppCompatActivity() {

    private lateinit var searchInput: EditText
    private lateinit var searchButton: Button
    private lateinit var suggestionsList: ListView
    private lateinit var entryWebView: WebView

    private val port: Long = OndictServerService.PORT

    // Injected once after G.Load() completes; reused for every entry render.
    private var css: String = ""

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // First run: no dictionaries configured yet — go to setup screen.
        if (!DictManager.hasConfig(this)) {
            SetupActivity.start(this)
        }

        setContentView(R.layout.activity_main)

        searchInput     = findViewById(R.id.searchInput)
        searchButton    = findViewById(R.id.searchButton)
        suggestionsList = findViewById(R.id.suggestionsList)
        entryWebView    = findViewById(R.id.entryWebView)

        setupWebView()
        setupSearch()

        // Start the foreground service that owns the Go HTTP server (needed
        // for sync endpoints). Dictionary loading happens inside StartServer
        // via sources.G.Load — once that completes the direct-query bindings
        // (Mobile.queryEntry / Mobile.getCSS) are ready to use.
        startForegroundService(Intent(this, OndictServerService::class.java))

        // Wait for the server to be ready (proxy for G.Load completing),
        // grab CSS once, apply sync config, then show the home screen.
        Thread {
            waitForServer()
            css = Mobile.getCSS()
            SyncManager.applyFromSettings(this)
            runOnUiThread { showWelcome() }
        }.start()
    }

    override fun onResume() {
        super.onResume()
        // Probe the server on every return to foreground. If it is not yet
        // responding (throttled after process survived in background), wait
        // and re-render the current entry once it's back.
        Thread {
            if (!isServerAlive()) {
                waitForServer()
                css = Mobile.getCSS()
                runOnUiThread {
                    val word = searchInput.text.toString().trim()
                    if (word.isNotEmpty()) lookupAndRender(word) else showWelcome()
                }
            }
        }.start()
    }

    // -------------------------------------------------------------------------
    // WebView setup
    // -------------------------------------------------------------------------

    private fun setupWebView() {
        entryWebView.settings.javaScriptEnabled = true

        entryWebView.webViewClient = object : WebViewClient() {

            override fun shouldOverrideUrlLoading(
                view: WebView,
                request: WebResourceRequest
            ): Boolean {
                val url = request.url
                return when {
                    // entry://word — cross-reference: look up new word directly.
                    url.scheme == "entry" -> {
                        val word = url.host ?: url.path?.trimStart('/') ?: return true
                        lookupAndRender(word)
                        searchInput.setText(word)
                        true
                    }
                    // sound://file.mp3 — audio: fetch bytes from Go, play natively.
                    url.scheme == "sound" -> {
                        val name = (url.host ?: "") + (url.path ?: "")
                        playAudio(name)
                        true
                    }
                    // /import, /sync — open native screens.
                    url.path == "/import" -> {
                        SetupActivity.start(this@MainActivity); true
                    }
                    url.path == "/sync" -> {
                        SyncSettingsActivity.start(this@MainActivity); true
                    }
                    else -> false
                }
            }

            // Intercept resource requests (images) so the WebView never
            // makes a real network request — serve everything from Go.
            override fun shouldInterceptRequest(
                view: WebView,
                request: WebResourceRequest
            ): WebResourceResponse? {
                val path = request.url.path?.trimStart('/') ?: return null
                val bytes = Mobile.getFile(path) ?: return null
                val mime = when {
                    path.endsWith(".mp3")                    -> "audio/mpeg"
                    path.endsWith(".png")                    -> "image/png"
                    path.endsWith(".jpg") || path.endsWith(".jpeg") -> "image/jpeg"
                    path.endsWith(".css")                    -> "text/css"
                    else                                     -> "application/octet-stream"
                }
                return WebResourceResponse(mime, "utf-8", ByteArrayInputStream(bytes))
            }
        }
    }

    // -------------------------------------------------------------------------
    // Search bar + autocomplete
    // -------------------------------------------------------------------------

    private fun setupSearch() {
        searchButton.setOnClickListener { submitSearch() }

        searchInput.setOnEditorActionListener { _, actionId, event ->
            if (actionId == EditorInfo.IME_ACTION_SEARCH ||
                (event?.keyCode == KeyEvent.KEYCODE_ENTER && event.action == KeyEvent.ACTION_DOWN)
            ) {
                submitSearch()
                true
            } else false
        }

        // Autocomplete — hit the /complete endpoint on the local server.
        searchInput.addTextChangedListener(object : android.text.TextWatcher {
            private val handler = android.os.Handler(android.os.Looper.getMainLooper())
            private var pending: Runnable? = null

            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {}
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {}
            override fun afterTextChanged(s: android.text.Editable?) {
                pending?.let { handler.removeCallbacks(it) }
                val prefix = s?.toString()?.trim() ?: return
                if (prefix.length < 2) {
                    suggestionsList.visibility = android.view.View.GONE
                    return
                }
                val r = Runnable { fetchSuggestions(prefix) }
                pending = r
                handler.postDelayed(r, 220)
            }
        })

        suggestionsList.setOnItemClickListener { _, _, position, _ ->
            val word = suggestionsList.adapter.getItem(position) as String
            searchInput.setText(word)
            suggestionsList.visibility = android.view.View.GONE
            lookupAndRender(word)
            hideKeyboard()
        }
    }

    private fun submitSearch() {
        val word = searchInput.text.toString().trim()
        if (word.isEmpty()) return
        suggestionsList.visibility = android.view.View.GONE
        lookupAndRender(word)
        hideKeyboard()
    }

    private fun fetchSuggestions(prefix: String) {
        Thread {
            try {
                val url = URL("http://127.0.0.1:$port/complete?prefix=${
                    java.net.URLEncoder.encode(prefix, "UTF-8")}&mode=fzf")
                val conn = url.openConnection() as HttpURLConnection
                conn.connectTimeout = 1000
                conn.readTimeout = 1000
                val body = conn.inputStream.bufferedReader().readText()
                conn.disconnect()
                // Parse JSON array ["word1","word2",...]
                val words = body.trim()
                    .removePrefix("[").removeSuffix("]")
                    .split(",")
                    .map { it.trim().removeSurrounding("\"") }
                    .filter { it.isNotEmpty() }
                runOnUiThread {
                    if (words.isEmpty()) {
                        suggestionsList.visibility = android.view.View.GONE
                    } else {
                        suggestionsList.adapter = ArrayAdapter(
                            this, android.R.layout.simple_list_item_1, words
                        )
                        suggestionsList.visibility = android.view.View.VISIBLE
                    }
                }
            } catch (_: Exception) {
                runOnUiThread { suggestionsList.visibility = android.view.View.GONE }
            }
        }.start()
    }

    // -------------------------------------------------------------------------
    // Entry rendering — direct Go call, no HTTP round-trip
    // -------------------------------------------------------------------------

    private fun lookupAndRender(word: String) {
        Thread {
            val html = Mobile.queryEntry(word)
            val page = buildEntryPage(word, html)
            runOnUiThread {
                entryWebView.loadDataWithBaseURL(
                    "https://ondict.local/",
                    page,
                    "text/html",
                    "utf-8",
                    null
                )
            }
        }.start()
    }

    /** Wraps the raw entry HTML fragment in a minimal page with injected CSS. */
    private fun buildEntryPage(word: String, entryHtml: String): String {
        val body = if (entryHtml.isEmpty()) {
            "<p style='color:grey;font-family:sans-serif'>No entry found for \"$word\".</p>"
        } else {
            entryHtml
        }
        return """<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
body { margin: 12px; font-family: serif; background: #fff; color: #111; }
@media (prefers-color-scheme: dark) {
  body { background: #1a1714; color: #f4ede2; }
}
$css
</style>
</head>
<body>
<article class="entry-card">$body</article>
<script>
// Audio: tap on [data-audio-src] elements sends a sound:// navigation
// which is intercepted natively by the WebViewClient.
document.addEventListener('click', function(e) {
  var el = e.target.closest('[data-audio-src]');
  if (!el) return;
  var src = el.getAttribute('data-audio-src');
  if (src) { window.location.href = src.replace(/^\//, 'sound://'); }
});
</script>
</body>
</html>"""
    }

    private fun showWelcome() {
        val page = """<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
body { margin: 40px 16px; font-family: sans-serif; text-align: center;
       background: #fff; color: #333; }
@media (prefers-color-scheme: dark) { body { background: #1a1714; color: #f4ede2; } }
p { color: #888; }
</style>
</head>
<body>
<h2>Ondict</h2>
<p>Type a word in the search bar above.</p>
</body>
</html>"""
        entryWebView.loadDataWithBaseURL(null, page, "text/html", "utf-8", null)
    }

    // -------------------------------------------------------------------------
    // Audio playback
    // -------------------------------------------------------------------------

    private fun playAudio(filename: String) {
        Thread {
            val bytes = Mobile.getFile(filename) ?: return@Thread
            try {
                val tmp = java.io.File(cacheDir, "audio_tmp.mp3")
                tmp.writeBytes(bytes)
                val mp = MediaPlayer().apply {
                    setAudioAttributes(
                        AudioAttributes.Builder()
                            .setUsage(AudioAttributes.USAGE_MEDIA)
                            .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
                            .build()
                    )
                    setDataSource(tmp.absolutePath)
                    prepare()
                    start()
                }
                mp.setOnCompletionListener { it.release() }
            } catch (e: Exception) {
                android.util.Log.e("Ondict", "audio playback error", e)
            }
        }.start()
    }

    // -------------------------------------------------------------------------
    // Helpers
    // -------------------------------------------------------------------------

    private fun isServerAlive(): Boolean = try {
        val conn = URL("http://127.0.0.1:$port").openConnection() as HttpURLConnection
        conn.connectTimeout = 500
        conn.readTimeout = 500
        val code = conn.responseCode
        conn.disconnect()
        code in 200..499
    } catch (_: Exception) { false }

    private fun waitForServer() {
        repeat(30) {
            if (isServerAlive()) return
            Thread.sleep(500)
        }
    }

    private fun hideKeyboard() {
        val imm = getSystemService(InputMethodManager::class.java)
        imm.hideSoftInputFromWindow(searchInput.windowToken, 0)
    }
}
