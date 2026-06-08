package com.ondict.app

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.os.IBinder
import mobile.Mobile

/**
 * Foreground service that owns the Go HTTP server thread.
 *
 * Running the server inside a foreground service (with a visible notification)
 * tells Android to keep this process alive at near-foreground priority. Without
 * this, Android can kill the process silently after a few minutes in the
 * background, causing the server to vanish and the WebView to hang on the next
 * query.
 *
 * Start with startForegroundService(intent) from MainActivity; the service
 * calls startForeground() immediately in onStartCommand so Android doesn't
 * ANR it for taking too long to promote itself.
 */
class OndictServerService : Service() {

    companion object {
        const val PORT = 1345L
        private const val CHANNEL_ID = "ondict_server"
        private const val NOTIFICATION_ID = 1
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        startForeground(NOTIFICATION_ID, buildNotification())

        Thread {
            Mobile.startServer(
                filesDir.absolutePath,
                cacheDir.absolutePath,
                PORT
            )
        }.start()

        // If Android kills and restarts the service, re-run onStartCommand.
        return START_STICKY
    }

    private fun buildNotification(): Notification {
        val nm = getSystemService(NotificationManager::class.java)
        if (nm.getNotificationChannel(CHANNEL_ID) == null) {
            nm.createNotificationChannel(
                NotificationChannel(
                    CHANNEL_ID,
                    "Dictionary Server",
                    NotificationManager.IMPORTANCE_LOW  // silent, no sound/vibration
                ).apply {
                    description = "Keeps the dictionary server running in the background"
                }
            )
        }
        return Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("Ondict")
            .setContentText("Dictionary server running")
            .setSmallIcon(android.R.drawable.ic_menu_search)
            .setOngoing(true)   // user cannot swipe it away
            .build()
    }
}
