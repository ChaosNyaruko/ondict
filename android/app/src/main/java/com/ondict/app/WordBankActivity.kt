package com.ondict.app

import android.app.AlertDialog
import android.content.Intent
import android.os.Bundle
import android.view.Gravity
import android.widget.*
import mobile.Mobile
import org.json.JSONArray

/**
 * Displays the user's saved word bank. Words are loaded via Mobile.wordbankList()
 * and displayed in a scrollable list. Each word can be tapped to look it up or
 * long-pressed to remove it.
 */
class WordBankActivity : SystemBarsAwareActivity() {

    private lateinit var listView: ListView
    private lateinit var emptyText: TextView
    private var words = mutableListOf<String>()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        title = "Word Bank"

        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.MATCH_PARENT
            )
        }

        emptyText = TextView(this).apply {
            text = "Your word bank is empty.\nLook up a word and tap \"Add to Word Bank\"."
            gravity = Gravity.CENTER
            textSize = 16f
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.MATCH_PARENT
            )
        }

        listView = ListView(this).apply {
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.MATCH_PARENT
            )
        }

        root.addView(emptyText)
        root.addView(listView)
        setContentView(root)

        listView.setOnItemClickListener { _, _, position, _ ->
            val word = words[position]
            // Return to MainActivity and trigger a lookup for this word.
            val intent = Intent(this, MainActivity::class.java).apply {
                putExtra(EXTRA_LOOKUP_WORD, word)
                flags = Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP
            }
            startActivity(intent)
        }

        listView.setOnItemLongClickListener { _, _, position, _ ->
            val word = words[position]
            AlertDialog.Builder(this)
                .setTitle("Remove \"$word\"?")
                .setMessage("Remove this word from your word bank.")
                .setPositiveButton("Remove") { _, _ ->
                    Thread {
                        Mobile.wordbankRemove(word)
                        runOnUiThread { loadWords() }
                    }.start()
                }
                .setNegativeButton("Cancel", null)
                .show()
            true
        }

        loadWords()
    }

    override fun onResume() {
        super.onResume()
        loadWords()
    }

    private fun loadWords() {
        Thread {
            val json = Mobile.wordbankList()
            val arr = JSONArray(json)
            val list = (0 until arr.length()).map { arr.getString(it) }
            runOnUiThread {
                words.clear()
                words.addAll(list)
                if (words.isEmpty()) {
                    emptyText.visibility = android.view.View.VISIBLE
                    listView.visibility = android.view.View.GONE
                } else {
                    emptyText.visibility = android.view.View.GONE
                    listView.visibility = android.view.View.VISIBLE
                    listView.adapter = ArrayAdapter(
                        this,
                        android.R.layout.simple_list_item_1,
                        words
                    )
                }
            }
        }.start()
    }

    companion object {
        const val EXTRA_LOOKUP_WORD = "lookup_word"

        fun start(from: android.content.Context) {
            from.startActivity(Intent(from, WordBankActivity::class.java))
        }
    }
}
