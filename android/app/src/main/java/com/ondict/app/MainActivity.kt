package com.ondict.app
import android.os.Bundle
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.activity.OnBackPressedCallback
import androidx.appcompat.app.AppCompatActivity
import mobile.Mobile
import java.net.HttpURLConnection
import java.net.URL
class MainActivity : AppCompatActivity() {
    private lateinit var webView: WebView
    private val port: Long = 1345
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
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
                return false
            }
            override fun onPageFinished(view: WebView, url: String) {
                // Keep the callback enabled only when there is history to go back to
                backCallback.isEnabled = view.canGoBack()
            }
        }
        setContentView(webView)

        // Start the Go HTTP server in a background thread, then load once ready
        Thread {
            Mobile.startServer(filesDir.absolutePath, cacheDir.absolutePath, port)
        }.start()
        Thread {
            waitForServer()
            runOnUiThread {
                webView.loadUrl("http://127.0.0.1:$port")
            }
        }.start()
    }
    private fun waitForServer() {
        val url = URL("http://127.0.0.1:$port")
        repeat(30) {
            try {
                val conn = url.openConnection() as HttpURLConnection
                conn.connectTimeout = 500
                conn.readTimeout = 500
                conn.responseCode // throws if not ready
                conn.disconnect()
                return // server is up
            } catch (e: Exception) {
                Thread.sleep(500)
            }
        }
    }
}
