package com.ondict.app

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.provider.OpenableColumns
import android.view.Gravity
import android.view.View
import android.widget.*
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity

/**
 * First-run setup screen for importing .mdx/.mdd dictionary files.
 * Launched automatically by MainActivity when no config exists.
 * Can also be launched manually via the "Import Dictionary" action.
 */
class SetupActivity : AppCompatActivity() {

    // Pending URI when user picked a file but hasn't confirmed the dict type
    private var pendingUri: Uri? = null
    private var pendingFileName: String? = null

    private lateinit var statusText: TextView
    private lateinit var dictListView: LinearLayout
    private lateinit var openButton: Button

    private val pickFile = registerForActivityResult(
        ActivityResultContracts.OpenDocument()
    ) { uri ->
        if (uri != null) onFilePicked(uri)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(48, 64, 48, 48)
        }

        // Title
        root.addView(TextView(this).apply {
            text = "Import Dictionary"
            textSize = 24f
            setPadding(0, 0, 0, 8)
        })

        // Subtitle
        root.addView(TextView(this).apply {
            text = "Select an .mdx or .mdd file from your device storage."
            textSize = 15f
            setPadding(0, 0, 0, 32)
        })

        // Import button
        openButton = Button(this).apply {
            text = "Pick .mdx / .mdd file"
            setOnClickListener {
                pickFile.launch(arrayOf("*/*"))
            }
        }
        root.addView(openButton)

        // Status text (shows progress/errors)
        statusText = TextView(this).apply {
            text = ""
            textSize = 14f
            setPadding(0, 16, 0, 0)
        }
        root.addView(statusText)

        // Divider
        root.addView(View(this).apply {
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT, 1
            ).also { it.setMargins(0, 32, 0, 24) }
            setBackgroundColor(0x22000000)
        })

        // Current dicts header
        root.addView(TextView(this).apply {
            text = "Configured dictionaries"
            textSize = 16f
            setPadding(0, 0, 0, 12)
        })

        // Dict list
        dictListView = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
        }
        root.addView(dictListView)

        // Done button
        root.addView(Button(this).apply {
            text = "Done"
            setOnClickListener { finish() }
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).also { it.setMargins(0, 32, 0, 0) }
        })

        val scroll = ScrollView(this)
        scroll.addView(root)
        setContentView(scroll)

        refreshDictList()
    }

    private fun onFilePicked(uri: Uri) {
        val fileName = resolveFileName(uri) ?: run {
            statusText.text = "Could not read file name."
            return
        }

        if (!fileName.endsWith(".mdx", ignoreCase = true) &&
            !fileName.endsWith(".mdd", ignoreCase = true)) {
            statusText.text = "Only .mdx and .mdd files are supported."
            return
        }

        if (fileName.endsWith(".mdd", ignoreCase = true)) {
            // MDD doesn't need a type — just copy it
            copyFile(uri, fileName, null)
            return
        }

        // For .mdx, ask for the dictionary type
        pendingUri = uri
        pendingFileName = fileName
        showTypeDialog(fileName)
    }

    private fun showTypeDialog(fileName: String) {
        val types = arrayOf(
            "LONGMAN/Easy",
            "LONGMAN5/Online",
            "OLD9",
            "Other (MDX)"
        )
        AlertDialog.Builder(this)
            .setTitle("Dictionary type for\n$fileName")
            .setItems(types) { _, which ->
                val type = if (which < 3) types[which] else "MDX"
                pendingUri?.let { copyFile(it, pendingFileName!!, type) }
                pendingUri = null
                pendingFileName = null
            }
            .setNegativeButton("Cancel", null)
            .show()
    }

    private fun copyFile(uri: Uri, fileName: String, dictType: String?) {
        statusText.text = "Copying $fileName…"
        openButton.isEnabled = false

        Thread {
            try {
                contentResolver.openInputStream(uri)!!.use { input ->
                    DictManager.importDict(
                        this,
                        fileName,
                        input,
                        dictType ?: "LONGMAN/Easy"
                    )
                }
                runOnUiThread {
                    statusText.text = "✓ $fileName imported successfully."
                    openButton.isEnabled = true
                    refreshDictList()
                }
            } catch (e: Exception) {
                runOnUiThread {
                    statusText.text = "Error: ${e.message}"
                    openButton.isEnabled = true
                }
            }
        }.start()
    }

    private fun refreshDictList() {
        dictListView.removeAllViews()
        val dicts = DictManager.listDicts(this)
        if (dicts.isEmpty()) {
            dictListView.addView(TextView(this).apply {
                text = "No dictionaries configured yet."
                textSize = 14f
            })
        } else {
            dicts.forEach { (name, type) ->
                val row = LinearLayout(this).apply {
                    orientation = LinearLayout.HORIZONTAL
                    gravity = Gravity.CENTER_VERTICAL
                    setPadding(0, 8, 0, 8)
                }
                row.addView(TextView(this).apply {
                    text = "$name  ($type)"
                    textSize = 14f
                    layoutParams = LinearLayout.LayoutParams(0,
                        LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                })
                row.addView(Button(this).apply {
                    text = "Remove"
                    textSize = 12f
                    setOnClickListener {
                        DictManager.removeDict(this@SetupActivity, name)
                        refreshDictList()
                    }
                })
                dictListView.addView(row)
            }
        }
    }

    private fun resolveFileName(uri: Uri): String? {
        contentResolver.query(uri, null, null, null, null)?.use { cursor ->
            val idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
            if (idx >= 0 && cursor.moveToFirst()) return cursor.getString(idx)
        }
        return uri.lastPathSegment
    }

    companion object {
        fun start(activity: Activity) {
            activity.startActivity(Intent(activity, SetupActivity::class.java))
        }
    }
}
