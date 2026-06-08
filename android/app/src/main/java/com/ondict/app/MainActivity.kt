package com.ondict.app

import android.media.AudioAttributes
import android.media.MediaPlayer
import android.os.Bundle
import android.view.KeyEvent
import android.view.View
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
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity
import mobile.Mobile
import org.json.JSONArray
import java.io.ByteArrayInputStream

class MainActivity : AppCompatActivity() {

    private lateinit var searchInput: EditText
    private lateinit var searchButton: Button
    private lateinit var suggestionsList: ListView
    private lateinit var entryWebView: WebView
    private lateinit var welcomeHint: TextView

    private val port: Long = OndictServerService.PORT  // kept for reference, server not started

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
        welcomeHint     = findViewById(R.id.welcomeHint)

        setupWebView()
        setupSearch()

        // Explicitly claim focus for the search input — prevents WebView
        // initialisation from stealing it on first layout pass.
        searchInput.requestFocus()

        // Initialise Go directly — load dictionaries and open local stores.
        // No HTTP server needed for query/render/complete/sync on Android.
        Thread {
            try {
                Mobile.init(filesDir.absolutePath, cacheDir.absolutePath)
            } catch (e: Exception) {
                android.util.Log.e("Ondict", "Mobile.init failed", e)
            }
            css = Mobile.getCSS()
            SyncManager.applyFromSettings(this)
        }.start()
    }

    override fun onResume() {
        super.onResume()
        // If the process was killed and recreated while backgrounded, css
        // will be empty. Re-fetch it and re-render the current word if any.
        if (css.isEmpty()) {
            Thread {
                css = Mobile.getCSS()
                runOnUiThread {
                    val word = searchInput.text.toString().trim()
                    if (word.isNotEmpty()) lookupAndRender(word)
                }
            }.start()
        }
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
                        searchInput.setText(word)
                        lookupAndRender(word)
                        true
                    }
                    // sound://file.mp3 — audio: fetch bytes from Go, play natively.
                    url.scheme == "sound" -> {
                        val name = (url.host ?: "") + (url.path ?: "")
                        playAudio(name)
                        true
                    }
                    url.path == "/import" -> {
                        SetupActivity.start(this@MainActivity); true
                    }
                    url.path == "/sync" -> {
                        SyncSettingsActivity.start(this@MainActivity); true
                    }
                    else -> false
                }
            }

            // Intercept resource requests (images) — serve from Go, no network.
            override fun shouldInterceptRequest(
                view: WebView,
                request: WebResourceRequest
            ): WebResourceResponse? {
                val path = request.url.path?.trimStart('/') ?: return null
                val bytes = Mobile.getFile(path) ?: return null
                val mime = when {
                    path.endsWith(".mp3")                           -> "audio/mpeg"
                    path.endsWith(".png")                           -> "image/png"
                    path.endsWith(".jpg") || path.endsWith(".jpeg") -> "image/jpeg"
                    path.endsWith(".css")                           -> "text/css"
                    else                                            -> "application/octet-stream"
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

        searchInput.addTextChangedListener(object : android.text.TextWatcher {
            private val handler = android.os.Handler(android.os.Looper.getMainLooper())
            private var pending: Runnable? = null

            override fun beforeTextChanged(s: CharSequence?, start: Int, count: Int, after: Int) {}
            override fun onTextChanged(s: CharSequence?, start: Int, before: Int, count: Int) {}
            override fun afterTextChanged(s: android.text.Editable?) {
                pending?.let { handler.removeCallbacks(it) }
                val prefix = s?.toString()?.trim() ?: return
                if (prefix.length < 2) {
                    suggestionsList.visibility = View.GONE
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
            // Dismiss suggestions before rendering so they don't linger.
            suggestionsList.visibility = View.GONE
            lookupAndRender(word)
            hideKeyboard()
        }
    }

    private fun submitSearch() {
        val word = searchInput.text.toString().trim()
        if (word.isEmpty()) return
        suggestionsList.visibility = View.GONE
        lookupAndRender(word)
        hideKeyboard()
    }

    private fun fetchSuggestions(prefix: String) {
        Thread {
            try {
                val json = Mobile.complete(prefix, 10)
                val arr = JSONArray(json)
                val words = (0 until arr.length()).map { arr.getString(it) }
                runOnUiThread {
                    if (words.isEmpty()) {
                        suggestionsList.visibility = View.GONE
                    } else {
                        suggestionsList.adapter = ArrayAdapter(
                            this, android.R.layout.simple_list_item_1, words
                        )
                        suggestionsList.visibility = View.VISIBLE
                    }
                }
            } catch (_: Exception) {
                runOnUiThread { suggestionsList.visibility = View.GONE }
            }
        }.start()
    }

    // -------------------------------------------------------------------------
    // Entry rendering — direct Go call, no HTTP round-trip
    // -------------------------------------------------------------------------

    private fun lookupAndRender(word: String) {
        suggestionsList.visibility = View.GONE
        welcomeHint.visibility = View.GONE
        entryWebView.visibility = View.VISIBLE
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
                // Return focus to the search input so the user can type
                // another word immediately without tapping the field again.
                searchInput.requestFocus()
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

    private fun hideKeyboard() {
        val imm = getSystemService(InputMethodManager::class.java)
        imm.hideSoftInputFromWindow(searchInput.windowToken, 0)
    }
}
