package app.radlog

import android.Manifest
import android.annotation.SuppressLint
import android.app.Activity
import android.app.NotificationManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.PowerManager
import android.provider.Settings
import android.view.WindowManager
import android.webkit.JavascriptInterface
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.webkit.WebViewAssetLoader
import org.json.JSONObject
import java.util.concurrent.Executors

/**
 * A tela: o HTML de assets/www num WebView (como o desktop, que usa o
 * WebView2/WebKit). A página chama o motor por RadLogNative.call e recebe
 * as respostas e os eventos por window.__rl.
 */
class MainActivity : Activity() {
    private lateinit var web: WebView
    private val pool = Executors.newFixedThreadPool(3)
    private var permCb: String? = null

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(saved: Bundle?) {
        super.onCreate(saved)
        try { Engine.start(this) } catch (_: Exception) { /* a tela mostra o erro */ }
        if (prefs().getBoolean("paired", false)) RadLogService.start(this)
        val loader = WebViewAssetLoader.Builder()
            .addPathHandler("/assets/", WebViewAssetLoader.AssetsPathHandler(this))
            .build()
        web = WebView(this)
        web.setBackgroundColor(0xFF0B0E10.toInt())
        web.settings.javaScriptEnabled = true
        web.settings.domStorageEnabled = true
        web.settings.allowFileAccess = false
        web.settings.allowContentAccess = false
        web.webViewClient = object : WebViewClient() {
            override fun shouldInterceptRequest(v: WebView, r: WebResourceRequest): WebResourceResponse? =
                loader.shouldInterceptRequest(r.url)

            override fun shouldOverrideUrlLoading(v: WebView, r: WebResourceRequest): Boolean {
                if (r.url.host == "appassets.androidplatform.net") return false
                startActivity(Intent(Intent.ACTION_VIEW, r.url)) // link de fora: navegador
                return true
            }
        }
        web.addJavascriptInterface(Native(), "RadLogNative")
        setContentView(web)
        UiBridge.attach(web)
        web.loadUrl("https://appassets.androidplatform.net/assets/www/index.html")
    }

    override fun onResume() {
        super.onResume()
        Engine.syncTimeZone()
        UiBridge.js("window.__rl&&__rl.event(\"resume\",{})")
    }

    override fun onDestroy() {
        UiBridge.detach(web)
        web.destroy()
        pool.shutdown()
        super.onDestroy()
    }

    @Deprecated("Activity sem AndroidX")
    override fun onBackPressed() {
        UiBridge.js("window.__rl&&__rl.back()")
    }

    private fun prefs() = getSharedPreferences("radlog", Context.MODE_PRIVATE)

    private fun done(id: String, json: String) = UiBridge.js("window.__rl&&__rl.done(${JSONObject.quote(id)},$json)")

    // ---------- permissões ----------

    private fun has(p: String) = checkSelfPermission(p) == PackageManager.PERMISSION_GRANTED

    private fun btPerms(): Array<String> = if (Build.VERSION.SDK_INT >= 31) {
        arrayOf(Manifest.permission.BLUETOOTH_SCAN, Manifest.permission.BLUETOOTH_CONNECT)
    } else {
        arrayOf(Manifest.permission.ACCESS_FINE_LOCATION)
    }

    private fun permState(): JSONObject {
        val pm = getSystemService(PowerManager::class.java)
        val nm = getSystemService(NotificationManager::class.java)
        return JSONObject()
            .put("bluetooth", btPerms().all { has(it) })
            .put("btOn", try { Engine.ble.enabled() } catch (_: Exception) { getSystemService(android.bluetooth.BluetoothManager::class.java)?.adapter?.isEnabled == true })
            .put("notifications", if (Build.VERSION.SDK_INT >= 33) has(Manifest.permission.POST_NOTIFICATIONS) else nm.areNotificationsEnabled())
            .put("battery", pm.isIgnoringBatteryOptimizations(packageName))
            .put("fullScreen", if (Build.VERSION.SDK_INT >= 34) nm.canUseFullScreenIntent() else true)
    }

    override fun onRequestPermissionsResult(code: Int, perms: Array<out String>, res: IntArray) {
        super.onRequestPermissionsResult(code, perms, res)
        permCb?.let { done(it, JSONObject().put("r", permState()).toString()) }
        permCb = null
    }

    @SuppressLint("BatteryLife") // o app precisa ler o dia todo com a tela apagada
    private fun askBattery() {
        val i = Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, Uri.parse("package:$packageName"))
        try { startActivity(i) } catch (_: Exception) {
            startActivity(Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS))
        }
    }

    inner class Native {
        /** Chama um método do motor; a resposta volta por __rl.done(id, …). */
        @JavascriptInterface
        fun call(id: String, method: String, args: String) {
            pool.execute {
                val out = try { Engine.call(method, args) } catch (e: Exception) {
                    JSONObject().put("e", e.message ?: e.toString()).toString()
                }
                done(id, out)
            }
        }

        /** Funções do Android (permissões, serviço, alarme). */
        @JavascriptInterface
        fun native(id: String, name: String, arg: String) {
            runOnUiThread {
                val ok = { r: Any? -> done(id, JSONObject().put("r", r ?: JSONObject.NULL).toString()) }
                when (name) {
                    "perms" -> ok(permState())
                    "askBluetooth" -> { permCb = id; requestPermissions(btPerms(), 1) }
                    "askNotifications" -> if (Build.VERSION.SDK_INT >= 33) {
                        permCb = id; requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 2)
                    } else ok(permState())
                    "askBattery" -> { askBattery(); ok(true) }
                    "askFullScreen" -> {
                        if (Build.VERSION.SDK_INT >= 34) startActivity(Intent(Settings.ACTION_MANAGE_APP_USE_FULL_SCREEN_INTENT, Uri.parse("package:$packageName")))
                        ok(true)
                    }
                    "openBluetooth" -> { startActivity(Intent(Settings.ACTION_BLUETOOTH_SETTINGS)); ok(true) }
                    "paired" -> {
                        prefs().edit().putBoolean("paired", arg == "true").apply()
                        if (arg == "true") RadLogService.start(this@MainActivity)
                        ok(true)
                    }
                    "isPaired" -> ok(prefs().getBoolean("paired", false))
                    "silence" -> { RadLogService.instance?.silence(); ok(true) }
                    "owner" -> { // "phone" pega o RC102, "pc" solta
                        val svc = RadLogService.instance
                        if (svc != null) svc.setOwner(arg == "phone") else if (arg == "phone") RadLogService.start(this@MainActivity)
                        ok(true)
                    }
                    "alarm" -> ok(JSONObject().put("sev", UiBridge.alarmSev ?: JSONObject.NULL).put("quiet", UiBridge.alarmQuiet))
                    "keepOn" -> {
                        if (arg == "true") window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
                        else window.clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
                        ok(true)
                    }
                    "info" -> ok(JSONObject().put("model", Build.MODEL).put("android", Build.VERSION.RELEASE)
                        .put("sdk", Build.VERSION.SDK_INT).put("app", packageManager.getPackageInfo(packageName, 0).versionName))
                    "testAlarm" -> { // arg: o som ("sirene", "bipes", "perigo")
                        if (UiBridge.alarmSev == null) {
                            AlarmSound.play(arg.ifEmpty { "sirene" }, false)
                            window.decorView.postDelayed({ if (UiBridge.alarmSev == null) AlarmSound.stop() }, 5000)
                        }
                        ok(true)
                    }
                    "exit" -> { finish(); ok(true) }
                    else -> done(id, JSONObject().put("e", "função desconhecida: $name").toString())
                }
            }
        }
    }
}
