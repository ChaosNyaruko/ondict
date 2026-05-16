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

    data class DictEntry(
        val name: String,
        val type: String,
        val enabled: Boolean,
        val hasFile: Boolean,   // .mdx exists on disk
        val hasMdd: Boolean     // .mdd exists on disk
    )

    fun dictsDir(context: Context): File =
        File(context.filesDir, "dicts").also { it.mkdirs() }

    fun configFile(context: Context): File =
        File(context.filesDir, "config.json")

    /** Returns true if at least one dictionary is configured and enabled. */
    fun hasConfig(context: Context): Boolean {
        val f = configFile(context)
        if (!f.exists()) return false
        return try {
            val obj = JSONObject(f.readText())
            val dicts = obj.getJSONArray("dicts")
            (0 until dicts.length()).any { i ->
                val d = dicts.getJSONObject(i)
                d.optBoolean("enabled", true)
            }
        } catch (e: Exception) {
            false
        }
    }

    /**
     * Returns all known dicts — both configured ones and any .mdx files
     * found on disk that aren't in config yet (auto-detected).
     */
    fun listAllDicts(context: Context): List<DictEntry> {
        val dir = dictsDir(context)
        val config = readConfig(context)
        val configuredDicts = config.getJSONArray("dicts")

        // Build map of configured entries: name -> DictEntry
        val configured = mutableMapOf<String, DictEntry>()
        for (i in 0 until configuredDicts.length()) {
            val obj = configuredDicts.getJSONObject(i)
            val name = obj.getString("name")
            configured[name] = DictEntry(
                name    = name,
                type    = obj.optString("type", "LONGMAN/Easy"),
                enabled = obj.optBoolean("enabled", true),
                hasFile = File(dir, "$name.mdx").exists(),
                hasMdd  = File(dir, "$name.mdd").exists()
            )
        }

        // Auto-detect .mdx files on disk not yet in config
        dir.listFiles { f -> f.extension.lowercase() == "mdx" }
            ?.forEach { file ->
                val name = file.nameWithoutExtension
                if (name !in configured) {
                    configured[name] = DictEntry(
                        name    = name,
                        type    = "LONGMAN/Easy",
                        enabled = false,   // not yet enabled — user must confirm
                        hasFile = true,
                        hasMdd  = File(dir, "$name.mdd").exists()
                    )
                }
            }

        // Return in config order first, then any newly detected ones at end
        val configuredNames = (0 until configuredDicts.length())
            .map { configuredDicts.getJSONObject(it).getString("name") }
        val rest = configured.keys.filter { it !in configuredNames }
        return (configuredNames + rest).mapNotNull { configured[it] }
    }

    /** Saves the full ordered list of DictEntry back to config.json. */
    fun saveDicts(context: Context, dicts: List<DictEntry>) {
        val config = readConfig(context)
        val arr = JSONArray()
        dicts.forEach { d ->
            arr.put(JSONObject().apply {
                put("name", d.name)
                put("type", d.type)
                put("enabled", d.enabled)
            })
        }
        config.put("dicts", arr)
        configFile(context).writeText(config.toString(2))
    }

    /**
     * Copies an input stream into dictsDir under [fileName],
     * then registers it in config.json with the given [dictType].
     * If a dict with the same name already exists it is overwritten.
     */
    fun importDict(
        context: Context,
        fileName: String,
        input: InputStream,
        dictType: String = "LONGMAN/Easy"
    ) {
        val dest = File(dictsDir(context), fileName)
        dest.outputStream().use { out -> input.copyTo(out) }

        // Only register .mdx files in config (.mdd is auto-paired)
        if (!fileName.endsWith(".mdx", ignoreCase = true)) return

        val baseName = fileName.removeSuffix(".mdx").removeSuffix(".MDX")
        val all = listAllDicts(context).toMutableList()

        // If already present, update type and enable it; otherwise append
        val idx = all.indexOfFirst { it.name == baseName }
        if (idx >= 0) {
            all[idx] = all[idx].copy(type = dictType, enabled = true, hasFile = true)
        } else {
            all.add(DictEntry(
                name    = baseName,
                type    = dictType,
                enabled = true,
                hasFile = true,
                hasMdd  = File(dictsDir(context), "$baseName.mdd").exists()
            ))
        }
        saveDicts(context, all)
    }

    /** Removes a dictionary entry from config (does not delete the file). */
    fun removeDict(context: Context, name: String) {
        val all = listAllDicts(context).filter { it.name != name }
        saveDicts(context, all)
    }

    fun readConfig(context: Context): JSONObject {
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
