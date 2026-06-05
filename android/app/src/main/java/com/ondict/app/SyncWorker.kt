package com.ondict.app

import android.content.Context
import androidx.work.Worker
import androidx.work.WorkerParameters
import mobile.Mobile

/**
 * Periodic sync worker. Scheduled by SyncManager when the user enables
 * auto-sync; cancelled when they disable it or unconfigure credentials.
 *
 * WorkManager can launch this Worker into a FRESH process — one in which
 * MainActivity has never run, StartServer has never been called, and the
 * Go-side syncClient is nil. Without an explicit bootstrap step, calling
 * Mobile.sync() would always fail with "sync not configured" and the Worker
 * would retry forever.
 *
 * The fix: call Mobile.initSyncOnly(configDir, cacheDir, baseURL, user, pass)
 * before each sync attempt. initSyncOnly skips dictionary loading and the
 * HTTP server; it only calls util.SetPaths + ConfigureSync, which is
 * everything Sync() needs. This mirrors what MainActivity does on a
 * normal launch but without the heavy path.
 */
class SyncWorker(
    context: Context,
    params: WorkerParameters
) : Worker(context, params) {

    override fun doWork(): Result {
        val ctx = applicationContext
        val s   = SyncSettings.read(ctx)

        if (!s.isConfigured) {
            // Nothing to do — settings were cleared while the work was queued.
            // Return success so WorkManager doesn't keep retrying a no-op.
            return Result.success()
        }

        // Initialise the Go-side paths + sync client every time we land
        // here. If StartServer already ran in this process the call is
        // cheap (ensureSharedStores is idempotent; ConfigureSync just
        // replaces the syncClient pointer). If the process is cold-started
        // by WorkManager this is the only bootstrap we get.
        val initErr = try {
            Mobile.initSyncOnly(
                ctx.filesDir.absolutePath,
                ctx.cacheDir.absolutePath,
                s.baseURL,
                s.username,
                s.password
            )
            null
        } catch (e: Exception) {
            e.message ?: e.toString()
        }

        if (initErr != null) {
            // Config/path error — not a transient network problem; retry
            // won't help until settings change, but we don't have a
            // "failure, don't retry" signal distinct from "please retry",
            // so return failure (WorkManager will not retry on FAILURE by
            // default when the policy is keep/replace).
            SyncSettings.recordResult(ctx, "init failed: $initErr")
            return Result.failure()
        }

        val r = SyncManager.syncOnceBlocking(ctx)
        return when (r) {
            is SyncManager.Result.Ok  -> Result.success()
            // Transient errors (bad network, server down) — let WorkManager retry.
            is SyncManager.Result.Err -> Result.retry()
        }
    }
}
