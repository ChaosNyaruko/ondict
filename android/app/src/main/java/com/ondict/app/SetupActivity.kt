package com.ondict.app

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.graphics.Typeface
import android.net.Uri
import android.os.Bundle
import android.provider.OpenableColumns
import android.view.Gravity
import android.view.View
import android.widget.*
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity

/**
 * Dictionary management screen:
 * - Auto-detects .mdx files already in storage
 * - Enable / disable individual dicts
 * - Reorder dicts (affects query priority)
 * - Import new .mdx / .mdd files via file picker
 */
class SetupActivity : AppCompatActivity() {

    private var dicts = mutableListOf<DictManager.DictEntry>()

    private lateinit var dictListView: LinearLayout
    private lateinit var statusText: TextView
    private lateinit var openButton: Button

    private var pendingUri: Uri? = null
    private var pendingFileName: String? = null

    private val pickFile = registerForActivityResult(
        ActivityResultContracts.OpenDocument()
    ) { uri -> if (uri != null) onFilePicked(uri) }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(48, 64, 48, 48)
        }

        // Title
        root.addView(TextView(this).apply {
            text = "Dictionaries"
            textSize = 24f
            setTypeface(null, Typeface.BOLD)
            setPadding(0, 0, 0, 6)
        })

        root.addView(TextView(this).apply {
            text = "Manage your dictionaries. Tap a toggle to enable/disable. Use ↑↓ to reorder."
            textSize = 13f
            setPadding(0, 0, 0, 24)
        })

        // Dict list
        dictListView = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
        }
        root.addView(dictListView)

        // Divider
        root.addView(divider())

        // Import section
        root.addView(TextView(this).apply {
            text = "Import dictionary file"
            textSize = 16f
            setTypeface(null, Typeface.BOLD)
            setPadding(0, 20, 0, 8)
        })

        root.addView(TextView(this).apply {
            text = "Pick an .mdx or .mdd file from your device storage."
            textSize = 13f
            setPadding(0, 0, 0, 12)
        })

        openButton = Button(this).apply {
            text = "Pick .mdx / .mdd file"
            setOnClickListener { pickFile.launch(arrayOf("*/*")) }
        }
        root.addView(openButton)

        statusText = TextView(this).apply {
            text = ""
            textSize = 13f
            setPadding(0, 8, 0, 0)
        }
        root.addView(statusText)

        // Done button
        root.addView(divider())
        root.addView(Button(this).apply {
            text = "Done"
            setOnClickListener {
                DictManager.saveDicts(this@SetupActivity, dicts)
                finish()
            }
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).also { it.setMargins(0, 20, 0, 0) }
        })

        val scroll = ScrollView(this)
        scroll.addView(root)
        setContentView(scroll)

        refreshDictList()
    }

    // -------------------------------------------------------------------------
    // Dict list UI
    // -------------------------------------------------------------------------

    private fun refreshDictList() {
        dicts = DictManager.listAllDicts(this).toMutableList()
        dictListView.removeAllViews()

        if (dicts.isEmpty()) {
            dictListView.addView(TextView(this).apply {
                text = "No dictionary files found. Import an .mdx file to get started."
                textSize = 13f
                setPadding(0, 0, 0, 12)
            })
            return
        }

        dicts.forEachIndexed { index, entry ->
            dictListView.addView(buildDictRow(entry, index))
        }
    }

    private fun buildDictRow(entry: DictManager.DictEntry, index: Int): View {
        val row = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(0, 8, 0, 8)
        }

        val topRow = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
        }

        // Up / Down buttons
        val upBtn = Button(this).apply {
            text = "↑"
            textSize = 12f
            isEnabled = index > 0
            layoutParams = LinearLayout.LayoutParams(80, LinearLayout.LayoutParams.WRAP_CONTENT)
            setOnClickListener { moveDict(index, index - 1) }
        }
        val downBtn = Button(this).apply {
            text = "↓"
            textSize = 12f
            isEnabled = index < dicts.size - 1
            layoutParams = LinearLayout.LayoutParams(80, LinearLayout.LayoutParams.WRAP_CONTENT)
            setOnClickListener { moveDict(index, index + 1) }
        }

        // Name + meta
        val nameCol = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            layoutParams = LinearLayout.LayoutParams(0,
                LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            setPadding(12, 0, 12, 0)
        }
        nameCol.addView(TextView(this).apply {
            text = entry.name
            textSize = 15f
            setTypeface(null, Typeface.BOLD)
        })
        nameCol.addView(TextView(this).apply {
            val mdd = if (entry.hasMdd) " + audio" else ""
            val missing = if (!entry.hasFile) "  ⚠ file missing" else ""
            text = "${entry.type}$mdd$missing"
            textSize = 12f
        })

        // Enable toggle
        val toggle = Switch(this).apply {
            isChecked = entry.enabled
            setOnCheckedChangeListener { _, checked ->
                dicts[index] = dicts[index].copy(enabled = checked)
            }
        }

        // Type button
        val typeBtn = Button(this).apply {
            text = "Type"
            textSize = 11f
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.WRAP_CONTENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            )
            setOnClickListener { showTypeDialog(index) }
        }

        // Remove button
        val removeBtn = Button(this).apply {
            text = "✕"
            textSize = 11f
            layoutParams = LinearLayout.LayoutParams(70,
                LinearLayout.LayoutParams.WRAP_CONTENT)
            setOnClickListener { confirmRemove(entry.name) }
        }

        topRow.addView(upBtn)
        topRow.addView(downBtn)
        topRow.addView(nameCol)
        topRow.addView(toggle)
        topRow.addView(typeBtn)
        topRow.addView(removeBtn)
        row.addView(topRow)

        // Thin separator
        row.addView(View(this).apply {
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT, 1
            ).also { it.setMargins(0, 8, 0, 0) }
            setBackgroundColor(0x18000000)
        })

        return row
    }

    private fun moveDict(from: Int, to: Int) {
        if (to < 0 || to >= dicts.size) return
        val tmp = dicts[from]
        dicts[from] = dicts[to]
        dicts[to] = tmp
        refreshFromMemory()
    }

    /** Refresh UI from in-memory list without re-reading disk. */
    private fun refreshFromMemory() {
        dictListView.removeAllViews()
        if (dicts.isEmpty()) {
            refreshDictList()
            return
        }
        dicts.forEachIndexed { index, entry ->
            dictListView.addView(buildDictRow(entry, index))
        }
    }

    private fun showTypeDialog(index: Int) {
        val types = arrayOf("LONGMAN/Easy", "LONGMAN5/Online", "OLD9", "MDX")
        AlertDialog.Builder(this)
            .setTitle("Dictionary type")
            .setItems(types) { _, which ->
                dicts[index] = dicts[index].copy(type = types[which])
                refreshFromMemory()
            }
            .setNegativeButton("Cancel", null)
            .show()
    }

    private fun confirmRemove(name: String) {
        AlertDialog.Builder(this)
            .setTitle("Remove \"$name\"?")
            .setMessage("Removes from config only. The file stays on device.")
            .setPositiveButton("Remove") { _, _ ->
                dicts.removeAll { it.name == name }
                refreshFromMemory()
            }
            .setNegativeButton("Cancel", null)
            .show()
    }

    // -------------------------------------------------------------------------
    // File import
    // -------------------------------------------------------------------------

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
            copyFile(uri, fileName, null)
            return
        }
        pendingUri = uri
        pendingFileName = fileName
        showImportTypeDialog(fileName)
    }

    private fun showImportTypeDialog(fileName: String) {
        val types = arrayOf("LONGMAN/Easy", "LONGMAN5/Online", "OLD9", "MDX")
        AlertDialog.Builder(this)
            .setTitle("Dictionary type for\n$fileName")
            .setItems(types) { _, which ->
                pendingUri?.let { copyFile(it, pendingFileName!!, types[which]) }
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
                    DictManager.importDict(this, fileName, input, dictType ?: "LONGMAN/Easy")
                }
                runOnUiThread {
                    statusText.text = "✓ $fileName imported."
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

    private fun resolveFileName(uri: Uri): String? {
        contentResolver.query(uri, null, null, null, null)?.use { cursor ->
            val idx = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
            if (idx >= 0 && cursor.moveToFirst()) return cursor.getString(idx)
        }
        return uri.lastPathSegment
    }

    // -------------------------------------------------------------------------
    // Helpers
    // -------------------------------------------------------------------------

    private fun divider() = View(this).apply {
        layoutParams = LinearLayout.LayoutParams(
            LinearLayout.LayoutParams.MATCH_PARENT, 1
        ).also { it.setMargins(0, 16, 0, 16) }
        setBackgroundColor(0x22000000)
    }

    companion object {
        fun start(activity: Activity) {
            activity.startActivity(Intent(activity, SetupActivity::class.java))
        }
    }
}
