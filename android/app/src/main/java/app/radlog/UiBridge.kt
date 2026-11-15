package app.radlog

import android.os.Handler
import android.os.Looper
import android.webkit.WebView
import org.json.JSONObject
import java.lang.ref.WeakReference

/** Leva eventos do motor e do serviço até a tela aberta (se houver). */
object UiBridge : Engine.Listener {
    private val main = Handler(Looper.getMainLooper())
    private var web: WeakReference<WebView>? = null
    @Volatile var alarmSev: String? = null
        private set
    @Volatile var alarmQuiet = false
        private set

    fun attach(w: WebView) {
        web = WeakReference(w)
        Engine.removeListener(this)
        Engine.addListener(this)
    }

    fun detach(w: WebView) {
        // outra tela pode já ter se ligado (a nova abre antes da velha fechar)
        if (web?.get() !== w) return
        web = null
        Engine.removeListener(this)
    }

    fun js(code: String) {
        main.post { web?.get()?.evaluateJavascript(code, null) }
    }

    override fun onEvent(name: String, data: String) {
        js("window.__rl&&__rl.event(${JSONObject.quote(name)},$data)")
    }

    fun alarm(sev: String?, quiet: Boolean) {
        if (sev == alarmSev && quiet == alarmQuiet) return
        alarmSev = sev
        alarmQuiet = quiet
        val s = if (sev == null) "null" else JSONObject.quote(sev)
        js("window.__rl&&__rl.event(\"alarm\",{\"sev\":$s,\"quiet\":$quiet})")
    }
}
