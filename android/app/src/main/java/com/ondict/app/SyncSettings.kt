package com.ondict.app

import android.content.Context
import android.content.SharedPreferences

/**
 * Persistent configuration for cloud sync (ADR 0001).
 *
 * Stored in a private SharedPreferences file. Password is currently in
 * plaintext on disk inside the app's private storage; for stronger
 * protection swap to androidx.security.EncryptedSharedPreferences (TODO).
 * Risk is acceptable for v1 because the data is self-hosted by the user.
 */
object SyncSettings {

    private const val PREFS_NAME      = "ondict_sync"
    private const val KEY_BASE_URL    = "base_url"
    private const val KEY_USERNAME    = "username"
    private const val KEY_PASSWORD    = "password"
    private const val KEY_AUTO_SYNC   = "auto_sync"
    private const val KEY_INTERVAL    = "auto_sync_interval_minutes"
    private const val KEY_LAST_SYNC   = "last_sync_epoch_ms"
    private const val KEY_LAST_STATUS = "last_sync_status"

    private const val DEFAULT_INTERVAL_MIN = 30L

    private fun prefs(context: Context): SharedPreferences =
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    /** Snapshot of every persisted field. */
    data class Snapshot(
        val baseURL: String,
        val username: String,
        val password: String,
        val autoSync: Boolean,
        val intervalMinutes: Long,
        val lastSyncEpochMs: Long,   // 0 = never
        val lastStatus: String       // human-readable result of the last sync
    ) {
        /** True iff every credential field is non-blank. */
        val isConfigured: Boolean
            get() = baseURL.isNotBlank() && username.isNotBlank() && password.isNotBlank()
    }

    fun read(context: Context): Snapshot {
        val p = prefs(context)
        return Snapshot(
            baseURL         = p.getString(KEY_BASE_URL, "").orEmpty(),
            username        = p.getString(KEY_USERNAME, "").orEmpty(),
            password        = p.getString(KEY_PASSWORD, "").orEmpty(),
            autoSync        = p.getBoolean(KEY_AUTO_SYNC, false),
            intervalMinutes = p.getLong(KEY_INTERVAL, DEFAULT_INTERVAL_MIN),
            lastSyncEpochMs = p.getLong(KEY_LAST_SYNC, 0L),
            lastStatus      = p.getString(KEY_LAST_STATUS, "").orEmpty()
        )
    }

    fun saveCredentials(
        context: Context,
        baseURL: String,
        username: String,
        password: String,
        autoSync: Boolean,
        intervalMinutes: Long
    ) {
        prefs(context).edit()
            .putString(KEY_BASE_URL,  baseURL.trim().trimEnd('/'))
            .putString(KEY_USERNAME,  username.trim())
            .putString(KEY_PASSWORD,  password)
            .putBoolean(KEY_AUTO_SYNC, autoSync)
            .putLong(KEY_INTERVAL,    intervalMinutes.coerceAtLeast(15L))
            .apply()
    }

    fun recordResult(context: Context, status: String, epochMs: Long = System.currentTimeMillis()) {
        prefs(context).edit()
            .putLong(KEY_LAST_SYNC, epochMs)
            .putString(KEY_LAST_STATUS, status)
            .apply()
    }

    fun clear(context: Context) {
        prefs(context).edit().clear().apply()
    }
}
