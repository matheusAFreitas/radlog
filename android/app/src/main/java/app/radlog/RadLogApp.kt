package app.radlog

import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager
import android.media.AudioAttributes
import android.media.RingtoneManager

class RadLogApp : Application() {
    companion object {
        const val CH_READING = "leitura"
        const val CH_ALARMS = "alarmes"
        const val CH_NOTICES = "avisos"
    }

    override fun onCreate() {
        super.onCreate()
        val nm = getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(NotificationChannel(CH_READING, getString(R.string.ch_reading), NotificationManager.IMPORTANCE_LOW).apply {
            description = getString(R.string.ch_reading_desc)
            setShowBadge(false)
        })
        // o som do alarme é o do app (AlarmSound), como no PC: o canal só vibra
        nm.createNotificationChannel(NotificationChannel(CH_ALARMS, getString(R.string.ch_alarms), NotificationManager.IMPORTANCE_HIGH).apply {
            description = getString(R.string.ch_alarms_desc)
            setSound(null, null)
            enableVibration(true)
            vibrationPattern = longArrayOf(0, 600, 300, 600, 300, 600)
            setBypassDnd(true)
            lockscreenVisibility = android.app.Notification.VISIBILITY_PUBLIC
        })
        nm.createNotificationChannel(NotificationChannel(CH_NOTICES, getString(R.string.ch_notices), NotificationManager.IMPORTANCE_DEFAULT).apply {
            description = getString(R.string.ch_notices_desc)
            setSound(RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION),
                AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_NOTIFICATION_EVENT).build())
            enableVibration(true)
            vibrationPattern = longArrayOf(0, 200)
        })
    }
}
