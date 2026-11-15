package app.radlog

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

/** Liga o serviço quando o celular liga (ou quando o app é atualizado). */
class BootReceiver : BroadcastReceiver() {
    override fun onReceive(ctx: Context, intent: Intent) {
        val prefs = ctx.getSharedPreferences("radlog", Context.MODE_PRIVATE)
        if (!prefs.getBoolean("paired", false)) return // primeira vez ainda não terminou
        RadLogService.start(ctx)
    }
}
