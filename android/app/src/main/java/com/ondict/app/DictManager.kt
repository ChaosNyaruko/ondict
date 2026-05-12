package com.ondict.app

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.io.InputStream

/**
 * Manages dictionary configuration and file storage.
 * Dicts are stored in filesDir/dicts/, config in filesDir/config.json.
 */
object DictManager {

    fun dictsDir(context: Context): File =
        File(context.filesDir, "dicts").also { it.mkdirs() }

    fun configFile(context: Context): File =
        File(context.filesDir, "config.json")

    /** Returns true if at least one dictionary is configured. */
    fun hasConfig(context: Context): Boolean {
        val f = configFile(context)
        if (!f.exists()) return false
        return try {
            val obj = JSONObject(f.readText())
            obj.getJSONArray("dicts").length() > 0
        } catch (e: Exception) {
            false
        }
    }

    /**
     * Copies an input stream into dictsDir under [fileName],
     * then registers it in config.json with the given [dictType].
     * If a dict with the same name already exists it is overwritten.
     */
    fun importDict(
        context: Context,
        fileName: String,       // e.g. "LDOCE.mdx"
        input: InputStream,
        dictType: String = "LONGMAN/Easy"
    ) {
        // Write the file
        val dest = File(dictsDir(context), fileName)
        dest.outputStream().use { out -> input.copyTo(out) }

        // Only register .mdx files in config (not .mdd — they're auto-detected)
        if (!fileName.endsWith(".mdx", ignoreCase = true)) return

        val baseName = fileName.removeSuffix(".mdx").removeSuffix(".MDX")
        val config = readConfig(context)
        val dicts = config.getJSONArray("dicts")

        // Remove existing entry with same name if present
        val updated = JSONArray()
        for (i in 0 until dicts.length()) {
            val entry = dicts.getJSONObject(i)
            if (entry.getString("name") != baseName) updated.put(entry)
        }
        updated.put(JSONObject().apply {
            put("name", baseName)
            put("type", dictType)
        })
        config.put("dicts", updated)
        configFile(context).writeText(config.toString(2))
    }

    /** Lists all configured dictionaries as (name, type) pairs. */
    fun listDicts(context: Context): List<Pair<String, String>> {
        val dicts = readConfig(context).getJSONArray("dicts")
        return (0 until dicts.length()).map { i ->
            val obj = dicts.getJSONObject(i)
            obj.getString("name") to obj.getString("type")
        }
    }

    /** Removes a dictionary entry from config (does not delete the file). */
    fun removeDict(context: Context, name: String) {
        val config = readConfig(context)
        val dicts = config.getJSONArray("dicts")
        val updated = JSONArray()
        for (i in 0 until dicts.length()) {
            val entry = dicts.getJSONObject(i)
            if (entry.getString("name") != name) updated.put(entry)
        }
        config.put("dicts", updated)
        configFile(context).writeText(config.toString(2))
    }

    private fun readConfig(context: Context): JSONObject {
        val f = configFile(context)
        return if (f.exists()) {
            try { JSONObject(f.readText()) } catch (e: Exception) { defaultConfig() }
        } else {
            defaultConfig()
        }
    }

    private fun defaultConfig() = JSONObject().apply {
        put("dicts", JSONArray())
        put("search", JSONObject().apply {
            put("definition_index", JSONObject().apply {
                put("tokenizer", "unicode61")
            })
        })
    }
}
