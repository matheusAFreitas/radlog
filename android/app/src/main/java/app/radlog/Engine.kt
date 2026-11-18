package app.radlog

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.util.Log
import androidx.browser.customtabs.CustomTabsIntent
import app.radlog.go.mobile.Host
import app.radlog.go.mobile.Mobile
import java.util.concurrent.CopyOnWriteArrayList

/**
 * O motor do RadLog (Go, pelo gomobile). É o mesmo App do desktop em "modo
 * celular": grava os minutos, calcula a dose e os alarmes e sincroniza pela
 * nuvem. Daqui saem os eventos para a tela (WebView) e para o serviço
 * (notificação e som do alarme).
 */
object Engine : Host {
    private const val TAG = "RadLog"

    fun interface Listener {
        fun onEvent(name: String, data: String)
    }

    private val listeners = CopyOnWriteArrayList<Listener>()
    private lateinit var app: Context
    lateinit var ble: Ble
        private set

    @Volatile
    var lastRc: String = ""
        private set

    @Synchronized
    fun start(ctx: Context) {
        syncTimeZone()
        if (Mobile.running()) return
        app = ctx.applicationContext
        ble = Ble(app)
        val dir = app.filesDir.resolve("RadLog")
        dir.mkdirs()
        val name = (Build.MODEL ?: "Android").take(40)
        Mobile.start(dir.absolutePath, name, this)
    }

    /** O Go no Android fica em UTC: passa o fuso do celular. */
    fun syncTimeZone() {
        try { Mobile.setTimeZone(java.util.TimeZone.getDefault().id) } catch (_: Exception) { }
    }

    fun call(method: String, args: String): String = Mobile.call(method, args)

    fun addListener(l: Listener) = listeners.add(l)
    fun removeListener(l: Listener) = listeners.remove(l)

    // ---- Host (chamado pelo Go, em threads do Go) ----

    override fun event(name: String, data: String) {
        if (name == "radiacode") lastRc = data
        for (l in listeners) {
            try {
                l.onEvent(name, data)
            } catch (e: Exception) {
                Log.w(TAG, "evento $name: $e")
            }
        }
    }

    override fun openURL(url: String) {
        // o login do Google: abre no navegador do sistema (aba personalizada),
        // que volta para o endereço local do motor (127.0.0.1) como no PC
        try {
            val i = CustomTabsIntent.Builder().build()
            i.intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            i.launchUrl(app, Uri.parse(url))
        } catch (e: Exception) {
            val i = Intent(Intent.ACTION_VIEW, Uri.parse(url)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            app.startActivity(i)
        }
    }

    override fun bleScan(ms: Long): String = ble.scan(ms)
    override fun bleConnect(addr: String) = ble.connect(addr)
    override fun bleWrite(chunk: ByteArray) = ble.write(chunk)
    override fun bleClose() = ble.close()
    override fun bleRssi(): Long = ble.rssi().toLong()
}
