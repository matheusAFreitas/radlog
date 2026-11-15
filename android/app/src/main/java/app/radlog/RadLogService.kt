package app.radlog

import android.app.Notification
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.graphics.Color
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.os.PowerManager
import android.util.Log
import org.json.JSONArray
import org.json.JSONObject
import java.text.NumberFormat
import java.util.Locale

/**
 * Serviço em primeiro plano: segura o motor ligado com a tela apagada (o
 * Android pede a notificação fixa), mostra a leitura nela e cuida do som e
 * da notificação dos alarmes, como a tela do desktop faz.
 */
class RadLogService : Service(), Engine.Listener {
    companion object {
        private const val TAG = "RadLog.svc"
        const val N_READING = 1
        const val N_ALARM = 2
        const val N_NOTICE = 3
        const val ACTION_SILENCE = "app.radlog.SILENCE"
        const val ACTION_RELEASE = "app.radlog.RELEASE"
        const val ACTION_TAKE = "app.radlog.TAKE"

        @Volatile var instance: RadLogService? = null
            private set

        fun start(ctx: Context) {
            val i = Intent(ctx, RadLogService::class.java)
            try {
                ctx.startForegroundService(i)
            } catch (e: Exception) {
                Log.w(TAG, "não deu para ligar o serviço: $e")
            }
        }
    }

    private val main = Handler(Looper.getMainLooper())
    private lateinit var nm: NotificationManager
    private var wake: PowerManager.WakeLock? = null
    private var lastPost = 0L
    private var pending: Runnable? = null

    // estado do alarme (o mesmo do alarmCtl do desktop)
    private var sev: String? = null // "alarme" ou "perigo"
    private var muted = false
    private var snoozeUntil = 0L
    private var attnShown = false

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        instance = this
        nm = getSystemService(NotificationManager::class.java)
        val n = reading(null)
        if (Build.VERSION.SDK_INT >= 34) {
            startForeground(N_READING, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_CONNECTED_DEVICE)
        } else {
            startForeground(N_READING, n)
        }
        try {
            Engine.start(this)
        } catch (e: Exception) {
            Log.e(TAG, "motor: $e")
        }
        Engine.addListener(this)
        val pm = getSystemService(PowerManager::class.java)
        // o motor lê o RC102 a cada segundo: sem isto o processador dorme
        // com a tela apagada e as leituras param
        wake = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "RadLog:leitura").apply {
            setReferenceCounted(false)
            acquire()
        }
        if (Engine.lastRc.isNotEmpty()) onEvent("radiacode", Engine.lastRc)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_SILENCE -> silence()
            ACTION_RELEASE -> setOwner(false)
            ACTION_TAKE -> setOwner(true)
        }
        return START_STICKY
    }

    override fun onTrimMemory(level: Int) {
        super.onTrimMemory(level)
        Thread { app.radlog.go.mobile.Mobile.flush() }.start()
    }

    override fun onDestroy() {
        Engine.removeListener(this)
        AlarmSound.stop()
        wake?.let { if (it.isHeld) it.release() }
        try { app.radlog.go.mobile.Mobile.flush() } catch (_: Exception) {}
        instance = null
        super.onDestroy()
    }

    /** Soltar (o PC é o dono) ou pegar o RC102 de volta. */
    fun setOwner(phone: Boolean) {
        Thread {
            val st = try { JSONObject(JSONObject(Engine.call("GetRadiacode", "[]")).optString("r", "{}")) } catch (_: Exception) { JSONObject() }
            val ble = st.optString("ble", "")
            Engine.call("SetRadiacode", JSONArray().put(phone).put(ble).toString())
            if (!phone) Engine.ble.close()
        }.start()
    }

    // ---------- eventos do motor ----------

    override fun onEvent(name: String, data: String) {
        if (name != "radiacode") return
        val st = try { JSONObject(data) } catch (_: Exception) { return }
        main.post { onRc(st) }
    }

    private fun onRc(st: JSONObject) {
        alarm(st)
        // a notificação fixa: no máximo uma por segundo
        val now = System.currentTimeMillis()
        pending?.let { main.removeCallbacks(it) }
        val r = Runnable { lastPost = System.currentTimeMillis(); nm.notify(N_READING, reading(st)) }
        pending = r
        if (now - lastPost >= 1000) r.run() else main.postDelayed(r, 1000 - (now - lastPost))
    }

    private fun fmt(v: Double): String {
        val nf = NumberFormat.getNumberInstance(Locale.getDefault())
        val d = when {
            v >= 100 -> 0
            v >= 10 -> 1
            v >= 1 -> 2
            else -> 3
        }
        val dd = if (v < 1 && v >= 0.1) 2 else d
        nf.minimumFractionDigits = dd
        nf.maximumFractionDigits = dd
        return nf.format(v)
    }

    private fun levelOf(st: JSONObject): String = st.optString("alertLevel", "")

    private fun color(lv: String) = when (lv) {
        "perigo" -> 0xFFF0656A.toInt()
        "alarme" -> 0xFFF28C38.toInt()
        "atencao" -> 0xFFF5B84A.toInt()
        else -> 0xFF6EE7A0.toInt()
    }

    private fun lvName(lv: String) = getString(when (lv) {
        "perigo" -> R.string.lv_danger
        "alarme" -> R.string.lv_alarm
        "atencao" -> R.string.lv_attn
        else -> R.string.lv_safe
    })

    private fun openApp(): PendingIntent = PendingIntent.getActivity(this, 0,
        Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)

    private fun action(act: String, req: Int): PendingIntent = PendingIntent.getBroadcast(this, req,
        Intent(this, ActionReceiver::class.java).setAction(act), PendingIntent.FLAG_IMMUTABLE)

    private fun reading(st: JSONObject?): Notification {
        val b = Notification.Builder(this, RadLogApp.CH_READING)
            .setSmallIcon(R.drawable.ic_stat)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setShowWhen(false)
            .setContentIntent(openApp())
            .setCategory(Notification.CATEGORY_STATUS)
        if (st == null) {
            return b.setContentTitle(getString(R.string.n_title)).setContentText(getString(R.string.n_searching)).build()
        }
        val state = st.optString("state")
        val off = st.optBoolean("off") || state == "off"
        val today = if (st.optBoolean("todayDevOK")) st.optDouble("todayDev", 0.0) else st.optDouble("today", 0.0)
        val todayTxt = getString(R.string.n_today, fmt(today))
        when {
            off -> {
                b.setContentTitle(getString(R.string.n_off)).setContentText(todayTxt).setColor(Color.GRAY)
                b.addAction(Notification.Action.Builder(null, getString(R.string.act_take), action(ACTION_TAKE, 3)).build())
            }
            state == "conectado" -> {
                val lv = levelOf(st)
                b.setContentTitle("${fmt(st.optDouble("usvh", 0.0))} µSv/h · ${lvName(lv)}")
                    .setContentText(todayTxt)
                    .setColor(color(lv))
                b.addAction(Notification.Action.Builder(null, getString(R.string.act_release), action(ACTION_RELEASE, 2)).build())
            }
            else -> {
                val t = when (state) {
                    "conectando" -> getString(R.string.n_connecting)
                    "erro" -> getString(R.string.n_error, st.optString("err"))
                    else -> getString(R.string.n_searching)
                }
                b.setContentTitle(getString(R.string.n_title)).setContentText(t)
                b.addAction(Notification.Action.Builder(null, getString(R.string.act_release), action(ACTION_RELEASE, 2)).build())
            }
        }
        return b.build()
    }

    // ---------- alarme (igual ao do PC) ----------

    private fun settings(): JSONObject = try {
        JSONObject(JSONObject(Engine.call("GetSettings", "[]")).optString("r", "{}"))
    } catch (_: Exception) { JSONObject() }

    private fun alarm(st: JSONObject) {
        val lv = if (st.optString("state") == "conectado") levelOf(st) else ""
        val now = System.currentTimeMillis()
        val newSev = when (lv) { "perigo" -> "perigo"; "alarme" -> "alarme"; else -> null }
        if (newSev != sev) {
            val was = sev
            sev = newSev
            when {
                newSev == null -> { muted = false; snoozeUntil = 0 }
                newSev == "perigo" && was != "perigo" -> snoozeUntil = 0 // perigo novo toca mesmo silenciado
                newSev == "alarme" && was == "perigo" -> snoozeUntil = 0
                was == null -> muted = false
            }
        }
        // atenção: um aviso curto por episódio
        if (lv == "atencao" && !attnShown) {
            attnShown = true
            notice(st)
        } else if (lv == "") {
            attnShown = false
            nm.cancel(N_NOTICE)
        }
        if (sev == null) {
            AlarmSound.stop()
            nm.cancel(N_ALARM)
            UiBridge.alarm(null, false)
            return
        }
        val usv = st.optDouble("usvh", 0.0)
        val lim = st.optJSONObject("limits")?.optDouble("alarm", 0.0) ?: 0.0
        val fast = sev == "alarme" && lim > 0 && usv >= lim * 2
        val quiet = if (sev == "perigo") snoozeUntil > now else muted
        if (quiet) {
            AlarmSound.stop()
        } else {
            val sound = if (sev == "perigo") "perigo" else settings().optString("alarmSound", "sirene").ifEmpty { "sirene" }
            AlarmSound.play(sound, fast)
        }
        alarmNote(st, quiet)
        UiBridge.alarm(sev, quiet)
    }

    private fun alarmNote(st: JSONObject, quiet: Boolean) {
        val v = fmt(st.optDouble("usvh", 0.0))
        val title = getString(if (sev == "perigo") R.string.a_danger_title else R.string.a_alarm_title, v)
        val b = Notification.Builder(this, RadLogApp.CH_ALARMS)
            .setSmallIcon(R.drawable.ic_stat)
            .setContentTitle(title)
            .setContentText(getString(R.string.a_text))
            .setColor(color(sev ?: "alarme"))
            .setCategory(Notification.CATEGORY_ALARM)
            .setOnlyAlertOnce(true)
            .setOngoing(true)
            .setContentIntent(openApp())
            .setFullScreenIntent(openApp(), true)
        if (!quiet) b.addAction(Notification.Action.Builder(null, getString(R.string.act_silence), action(ACTION_SILENCE, 1)).build())
        nm.notify(N_ALARM, b.build())
    }

    private fun notice(st: JSONObject) {
        val v = fmt(st.optDouble("usvh", 0.0))
        nm.notify(N_NOTICE, Notification.Builder(this, RadLogApp.CH_NOTICES)
            .setSmallIcon(R.drawable.ic_stat)
            .setContentTitle(getString(R.string.a_attn_title, v))
            .setColor(color("atencao"))
            .setAutoCancel(true)
            .setContentIntent(openApp())
            .build())
    }

    /** Silenciar: o alarme cala até acabar; o perigo cala pelo tempo dos ajustes. */
    fun silence() {
        main.post {
            when (sev) {
                "perigo" -> {
                    val min = settings().optJSONObject("levels")?.optInt("snoozeMin", 5)?.takeIf { it > 0 } ?: 5
                    snoozeUntil = System.currentTimeMillis() + min * 60_000L
                    main.postDelayed({ Engine.lastRc.takeIf { it.isNotEmpty() }?.let { onEvent("radiacode", it) } }, min * 60_000L + 500)
                }
                "alarme" -> muted = !muted // de novo: liga o som (como no PC)
            }
            Engine.lastRc.takeIf { it.isNotEmpty() }?.let { try { alarm(JSONObject(it)) } catch (_: Exception) {} }
        }
    }
}
