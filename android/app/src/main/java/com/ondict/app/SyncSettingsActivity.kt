package com.ondict.app

import android.app.Activity
import android.content.Intent
import android.graphics.Typeface
import android.os.Bundle
import android.text.InputType
import android.view.Gravity
import android.view.View
import android.widget.*
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * Cloud-sync settings screen. Mirrors the in-code-built layout style of
 * SetupActivity (no XML).
 *
 * Workflow:
 *   1. User fills in base URL / username / password.
 *   2. Tap "Save" to persist + push to mobile.Mobile.initSyncOnly.
 *   3. Tap "Sync now" to trigger an immediate one-shot cycle on a
 *      background thread; result is shown in statusText.
 */
class SyncSettingsActivity : SystemBarsAwareActivity() {

    private lateinit var urlField: EditText
    private lateinit var userField: EditText
    private lateinit var passField: EditText
    private lateinit var autoToggle: Switch
    private lateinit var intervalField: EditText
    private lateinit var statusText: TextView
    private lateinit var lastSyncText: TextView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(48, 64, 48, 48)
        }

        root.addView(TextView(this).apply {
            text = "Cloud Sync"
            textSize = 24f
            setTypeface(null, Typeface.BOLD)
            setPadding(0, 0, 0, 6)
        })
        root.addView(TextView(this).apply {
            text = "Connect to a self-hosted ondict sync server to keep your Word Bank and history in sync across devices."
            textSize = 13f
            setPadding(0, 0, 0, 24)
        })

        val current = SyncSettings.read(this)

        // ----- Server URL -----
        root.addView(label("Server URL"))
        urlField = EditText(this).apply {
            hint = "https://sync.example.com"
            setText(current.baseURL)
            inputType = InputType.TYPE_TEXT_VARIATION_URI
            isSingleLine = true
        }
        root.addView(urlField)

        // ----- Username -----
        root.addView(label("Username"))
        userField = EditText(this).apply {
            setText(current.username)
            inputType = InputType.TYPE_CLASS_TEXT
            isSingleLine = true
        }
        root.addView(userField)

        // ----- Password -----
        root.addView(label("Password"))
        passField = EditText(this).apply {
            setText(current.password)
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
            isSingleLine = true
        }
        root.addView(passField)

        // ----- Auto-sync toggle -----
        root.addView(divider())

        val autoRow = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            setPadding(0, 12, 0, 12)
        }
        autoRow.addView(TextView(this).apply {
            text = "Auto-sync in background"
            textSize = 15f
            layoutParams = LinearLayout.LayoutParams(
                0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f
            )
        })
        autoToggle = Switch(this).apply {
            isChecked = current.autoSync
        }
        autoRow.addView(autoToggle)
        root.addView(autoRow)

        // ----- Interval -----
        root.addView(label("Interval (minutes, min 15)"))
        intervalField = EditText(this).apply {
            setText(current.intervalMinutes.toString())
            inputType = InputType.TYPE_CLASS_NUMBER
            isSingleLine = true
        }
        root.addView(intervalField)

        // ----- Buttons -----
        root.addView(divider())

        val btnRow = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
        }
        btnRow.addView(Button(this).apply {
            text = "Save"
            layoutParams = LinearLayout.LayoutParams(
                0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f
            )
            setOnClickListener { onSave() }
        })
        btnRow.addView(Button(this).apply {
            text = "Sync now"
            layoutParams = LinearLayout.LayoutParams(
                0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f
            ).also { it.setMargins(16, 0, 0, 0) }
            setOnClickListener { onSyncNow() }
        })
        root.addView(btnRow)

        // ----- Status -----
        statusText = TextView(this).apply {
            text = if (current.autoSyncSuspended && current.suspendReason.isNotBlank()) {
                "Auto-sync paused: ${current.suspendReason}"
            } else {
                ""
            }
            textSize = 13f
            setPadding(0, 16, 0, 0)
        }
        root.addView(statusText)

        lastSyncText = TextView(this).apply {
            text = formatLastSync(current)
            textSize = 12f
            setPadding(0, 8, 0, 0)
        }
        root.addView(lastSyncText)

        // ----- Done -----
        root.addView(divider())
        root.addView(Button(this).apply {
            text = "Done"
            setOnClickListener { finish() }
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).also { it.setMargins(0, 20, 0, 0) }
        })

        val scroll = ScrollView(this)
        scroll.addView(root)
        setContentView(scroll)
    }

    // -------------------------------------------------------------------------
    // Actions
    // -------------------------------------------------------------------------

    /**
     * Persists the form values and pushes credentials to the Go sync client.
     * Returns true on success, false on failure (statusText is already
     * updated by this function).
     */
    private fun onSave(): Boolean {
        val url      = urlField.text.toString()
        val user     = userField.text.toString()
        val pass     = passField.text.toString()
        val auto     = autoToggle.isChecked
        val interval = intervalField.text.toString().toLongOrNull() ?: 30L

        SyncSettings.saveCredentials(this, url, user, pass, auto, interval)

        val err = SyncManager.applyFromSettings(this)
        return if (err == null) {
            statusText.text = "✓ Saved."
            true
        } else {
            statusText.text = "✗ Configure failed: $err"
            false
        }
    }

    private fun onSyncNow() {
        // Save first; bail immediately if configuration failed so we don't
        // mask the error message or attempt a sync with stale/nil credentials.
        if (!onSave()) return

        statusText.text = "Syncing…"

        Thread {
            val r = SyncManager.syncOnceBlocking(this)
            if (r is SyncManager.Result.Err && !r.transient) {
                SyncManager.suspendAutoSync(this, r.message)
            }
            runOnUiThread {
                statusText.text = r.message
                lastSyncText.text = formatLastSync(SyncSettings.read(this))
            }
        }.start()
    }

    // -------------------------------------------------------------------------
    // Helpers
    // -------------------------------------------------------------------------

    private fun label(text: String) = TextView(this).apply {
        this.text = text
        textSize = 13f
        setTypeface(null, Typeface.BOLD)
        setPadding(0, 16, 0, 4)
    }

    private fun divider() = View(this).apply {
        layoutParams = LinearLayout.LayoutParams(
            LinearLayout.LayoutParams.MATCH_PARENT, 1
        ).also { it.setMargins(0, 16, 0, 16) }
        setBackgroundColor(0x22000000)
    }

    private fun formatLastSync(s: SyncSettings.Snapshot): String {
        if (s.lastSyncEpochMs == 0L) return "Last sync: never"
        val fmt = SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.getDefault())
        val when_ = fmt.format(Date(s.lastSyncEpochMs))
        return "Last sync: $when_\n${s.lastStatus}"
    }

    companion object {
        fun start(activity: Activity) {
            activity.startActivity(Intent(activity, SyncSettingsActivity::class.java))
        }
    }
}
