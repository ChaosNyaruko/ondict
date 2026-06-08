package com.ondict.app

import android.content.Intent
import android.os.Bundle
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.activity.OnBackPressedCallback
import androidx.appcompat.app.AppCompatActivity
import java.net.HttpURLConnection
import java.net.URL

class MainActivity : AppCompatActivity() {
    private lateinit var webView: WebView
    private val port: Long = OndictServerService.PORT

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // First run: no dictionaries configured yet — go to setup screen
        if (!DictManager.hasConfig(this)) {
            SetupActivity.start(this)
            // Don't finish() — stay in back stack so returning from setup
            // lands here and starts the server normally
        }

        webView = WebView(this)
        webView.settings.javaScriptEnabled = true
        webView.settings.domStorageEnabled = true

        // Handle back gesture/button: navigate WebView history before closing
        val backCallback = object : OnBackPressedCallback(false) {
            override fun handleOnBackPressed() {
                webView.goBack()
            }
        }
        onBackPressedDispatcher.addCallback(this, backCallback)

        webView.webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(
                view: WebView,
                request: WebResourceRequest
            ): Boolean {
                // Intercept the /import path to open SetupActivity
                if (request.url.path == "/import") {
                    SetupActivity.start(this@MainActivity)
                    return true
                }
                // Intercept /sync to open SyncSettingsActivity. Lets the
                // existing in-page nav (e.g. links in templates) reach
                // the sync settings screen without needing native menu UI.
                if (request.url.path == "/sync") {
                    SyncSettingsActivity.start(this@MainActivity)
                    return true
                }
                return false
            }
            override fun onPageFinished(view: WebView, url: String) {
                backCallback.isEnabled = view.canGoBack()
            }
        }
        setContentView(webView)

        // Start the Go HTTP server as a foreground service so Android keeps
        // the process alive when the app moves to the background.
        startForegroundService(Intent(this, OndictServerService::class.java))

        // Wait for the server to be ready, then apply sync config and load.
        Thread {
            waitForServer()
            SyncManager.applyFromSettings(this)
            runOnUiThread {
                webView.loadUrl("http://127.0.0.1:$port")
            }
        }.start()
    }

    override fun onResume() {
        super.onResume()
        // Each time the app returns to the foreground, probe the server.
        // If it is not responding (e.g. it was briefly throttled after the
        // process survived in the background), reload the WebView so the
        // user gets a fresh page instead of a stale or broken one.
        Thread {
            if (!isServerAlive()) {
                // Server not yet ready — wait for it, then reload.
                waitForServer()
                runOnUiThread { webView.reload() }
            }
        }.start()
    }

    // ---------------------------------------------------------------------------
    // Helpers
    // ---------------------------------------------------------------------------

    /** Returns true if the server responds within a short timeout. */
    private fun isServerAlive(): Boolean {
        return try {
            val conn = URL("http://127.0.0.1:$port").openConnection() as HttpURLConnection
            conn.connectTimeout = 500
            conn.readTimeout = 500
            val code = conn.responseCode
            conn.disconnect()
            code in 200..499
        } catch (e: Exception) {
            false
        }
    }

    /** Blocks until the server responds or the retry limit is reached. */
    private fun waitForServer() {
        val url = URL("http://127.0.0.1:$port")
        repeat(30) {
            try {
                val conn = url.openConnection() as HttpURLConnection
                conn.connectTimeout = 500
                conn.readTimeout = 500
                conn.responseCode
                conn.disconnect()
                return
            } catch (e: Exception) {
                Thread.sleep(500)
            }
        }
    }
}
