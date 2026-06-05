package com.ondict.app

import android.content.Context
import androidx.work.Worker
import androidx.work.WorkerParameters

/**
 * Periodic sync worker. Scheduled by SyncManager when the user enables
 * auto-sync; cancelled when they disable it or unconfigure credentials.
 *
 * Network connectivity is enforced as a Constraint so the OS doesn't
 * wake us up to fail. Beyond that we just delegate to syncOnceBlocking
 * and trust WorkManager's exponential backoff to retry on Result.retry().
 */
class SyncWorker(
    context: Context,
    params: WorkerParameters
) : Worker(context, params) {

    override fun doWork(): Result {
        val r = SyncManager.syncOnceBlocking(applicationContext)
        return when (r) {
            is SyncManager.Result.Ok  -> Result.success()
            // Transient errors (bad network, server down) — let WorkManager retry.
            is SyncManager.Result.Err -> Result.retry()
        }
    }
}
