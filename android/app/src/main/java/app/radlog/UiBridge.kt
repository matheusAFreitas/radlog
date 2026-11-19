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
    // 19/11: tela fechada (celular no bolso) não recebe os eventos de cada
    // segundo: a página redesenhava o Agora inteiro sem ninguém olhar e
    // gastava CPU. Ao voltar, "resume" relê tudo.
    @Volatile var paused = false

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
        if (paused) return
        js("window.__rl&&__rl.event(${JSONObject.quote(name)},$data)")
    }

    // de volta à tela: o alarme de agora (pode ter mudado com a tela fechada)
    fun resumed() {
        paused = false
        val s = alarmSev?.let { JSONObject.quote(it) } ?: "null"
        js("window.__rl&&__rl.event(\"alarm\",{\"sev\":$s,\"quiet\":$alarmQuiet})")
        js("window.__rl&&__rl.event(\"resume\",{})")
    }

    fun alarm(sev: String?, quiet: Boolean) {
        if (sev == alarmSev && quiet == alarmQuiet) return
        alarmSev = sev
        alarmQuiet = quiet
        if (paused) return
        val s = if (sev == null) "null" else JSONObject.quote(sev)
        js("window.__rl&&__rl.event(\"alarm\",{\"sev\":$s,\"quiet\":$quiet})")
    }
}
