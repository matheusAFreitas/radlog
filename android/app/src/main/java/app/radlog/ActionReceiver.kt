package app.radlog

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

/** Os botões das notificações (Silenciar, Soltar e Pegar o RC102). */
class ActionReceiver : BroadcastReceiver() {
    override fun onReceive(ctx: Context, intent: Intent) {
        val svc = RadLogService.instance
        when (intent.action) {
            RadLogService.ACTION_SILENCE -> svc?.silence()
            RadLogService.ACTION_RELEASE -> svc?.setOwner(false)
            RadLogService.ACTION_TAKE -> svc?.setOwner(true)
        }
    }
}
