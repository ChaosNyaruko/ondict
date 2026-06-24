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
        if (s.autoSyncSuspended) {
            // A previous permanent error already cancelled the unique periodic
            // work. This handles any stale invocation that was already queued.
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
            // Config/path error is not a transient network problem. Cancel
            // the unique periodic work and leave the user's autoSync choice
            // intact; saving settings clears the suspension.
            val reason = "init failed: $initErr"
            SyncSettings.recordResult(ctx, reason)
            SyncManager.suspendAutoSync(ctx, reason)
            return Result.failure()
        }

        val r = SyncManager.syncOnceBlocking(ctx)
        return when (r) {
            is SyncManager.Result.Ok  -> Result.success()
            is SyncManager.Result.Err ->
                // Only retry for transient errors (network timeouts, 5xx).
                // Permanent errors (4xx: wrong password, wrong URL, bad
                // request) will not resolve without user action. Cancel the
                // unique periodic work but keep the user's autoSync preference
                // unchanged; a later Save clears the suspension and schedules
                // this worker again.
                if (r.transient) {
                    Result.retry()
                } else {
                    SyncManager.suspendAutoSync(ctx, r.message)
                    Result.failure()
                }
        }
    }
}
