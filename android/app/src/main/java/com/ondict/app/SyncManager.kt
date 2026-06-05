package com.ondict.app

import android.content.Context
import android.util.Log
import androidx.work.Constraints
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.NetworkType
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import mobile.Mobile
import java.util.concurrent.TimeUnit

/**
 * Bridges the Kotlin layer to the Go-side sync client (mobile.Mobile).
 *
 * Lifecycle:
 *   1. MainActivity calls [applyFromSettings] after Mobile.startServer
 *      returns — this loads any persisted credentials and pushes them
 *      down to the Go sync client.
 *   2. SyncSettingsActivity calls [applyFromSettings] again whenever
 *      the user saves new credentials.
 *   3. [syncOnceBlocking] runs a manual one-shot pull-then-push cycle.
 *      Safe to invoke from a background thread; never call it on the
 *      UI thread (Mobile.sync() does network I/O).
 *   4. WorkManager schedules [SyncWorker] for periodic background sync
 *      when the user enables auto-sync.
 */
object SyncManager {

    private const val TAG = "ondict.sync"
    private const val WORK_NAME = "ondict-periodic-sync"

    /**
     * Re-reads SyncSettings from disk and pushes the credentials to the
     * Go sync client (or disables it if any field is blank). Also
     * (un)schedules the periodic worker.
     *
     * Safe to call repeatedly. Returns null on success, or the underlying
     * error from Mobile.configureSync() formatted as a string.
     */
    fun applyFromSettings(context: Context): String? {
        val s = SyncSettings.read(context)
        return try {
            // Empty fields disable sync on the Go side without erroring.
            Mobile.configureSync(s.baseURL, s.username, s.password)
            scheduleOrCancelPeriodic(context, s)
            null
        } catch (e: Exception) {
            Log.w(TAG, "configureSync failed", e)
            e.message ?: e.toString()
        }
    }

    /**
     * Runs one pull-then-push cycle. Returns the success summary or an
     * error message; in both cases also persists the result into
     * SyncSettings so the UI can show "last sync: ..." text.
     */
    fun syncOnceBlocking(context: Context): Result {
        val out: Result = try {
            val summary = Mobile.sync()
            Result.Ok(summary ?: "sync ok")
        } catch (e: Exception) {
            Log.w(TAG, "sync failed", e)
            val errMsg = e.message ?: e.toString()
            // Classify the error so callers (e.g. SyncWorker) can decide
            // whether to retry. 4xx HTTP errors are permanent failures;
            // network timeouts and 5xx are transient.
            val transient = Mobile.isSyncTransient(errMsg)
            Result.Err(errMsg, transient)
        }
        SyncSettings.recordResult(context, out.message)
        return out
    }

    /** Schedules or cancels the WorkManager periodic worker per the snapshot. */
    private fun scheduleOrCancelPeriodic(context: Context, s: SyncSettings.Snapshot) {
        val wm = WorkManager.getInstance(context)
        if (!s.isConfigured || !s.autoSync) {
            wm.cancelUniqueWork(WORK_NAME)
            return
        }
        // Minimum periodic interval that WorkManager honours is 15 minutes.
        val minutes = s.intervalMinutes.coerceAtLeast(15L)
        val request = PeriodicWorkRequestBuilder<SyncWorker>(minutes, TimeUnit.MINUTES)
            .setConstraints(
                Constraints.Builder()
                    .setRequiredNetworkType(NetworkType.CONNECTED)
                    .build()
            )
            .build()
        wm.enqueueUniquePeriodicWork(WORK_NAME, ExistingPeriodicWorkPolicy.UPDATE, request)
    }

    sealed class Result(val message: String) {
        class Ok(message: String) : Result(message)
        /**
         * @param transient true if the error is worth retrying (network
         *   issue, server-side 5xx); false for permanent failures (4xx).
         */
        class Err(message: String, val transient: Boolean = true) : Result("error: $message")
    }
}
