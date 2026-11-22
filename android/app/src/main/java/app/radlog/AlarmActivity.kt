package app.radlog

import android.app.Activity
import android.app.KeyguardManager
import android.content.Intent
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.util.TypedValue
import android.view.Gravity
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView

/**
 * Tela do alarme por cima da tela de bloqueio. Até o dev.625 a MainActivity
 * inteira tinha showWhenLocked: com o celular bloqueado dava para usar o app
 * todo (22/11). Agora só esta tela aparece bloqueada: o alarme, Silenciar e
 * "Abrir o RadLog", que pede o desbloqueio antes de abrir o app.
 */
class AlarmActivity : Activity() {
    private val main = Handler(Looper.getMainLooper())
    private lateinit var title: TextView
    private lateinit var state: TextView
    private lateinit var silence: Button
    private lateinit var box: LinearLayout

    private val tick = object : Runnable {
        override fun run() {
            refresh()
            main.postDelayed(this, 1000)
        }
    }

    private fun dp(v: Int) = TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, v.toFloat(), resources.displayMetrics).toInt()

    private fun button(text: String, fill: Int, fg: Int) = Button(this).apply {
        this.text = text
        isAllCaps = false
        setTextColor(fg)
        setTextSize(TypedValue.COMPLEX_UNIT_SP, 18f)
        background = GradientDrawable().apply { cornerRadius = dp(12).toFloat(); setColor(fill) }
        minHeight = dp(56)
        layoutParams = LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, LinearLayout.LayoutParams.WRAP_CONTENT).apply { topMargin = dp(12) }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        box = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER
            setPadding(dp(24), dp(24), dp(24), dp(24))
            setBackgroundColor(0xFF0B0E10.toInt())
        }
        title = TextView(this).apply {
            setTextColor(Color.WHITE)
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 28f)
            typeface = Typeface.DEFAULT_BOLD
            gravity = Gravity.CENTER
        }
        val text = TextView(this).apply {
            setText(R.string.a_lock_text)
            setTextColor(0xFFC9D1D6.toInt())
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 17f)
            gravity = Gravity.CENTER
            setPadding(0, dp(12), 0, dp(24))
        }
        state = TextView(this).apply {
            setTextColor(0xFF9AA4AB.toInt())
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 15f)
            gravity = Gravity.CENTER
        }
        silence = button(getString(R.string.act_silence), 0xFF2A3238.toInt(), Color.WHITE)
        silence.setOnClickListener { RadLogService.instance?.silence(); main.postDelayed({ refresh() }, 300) }
        val open = button(getString(R.string.act_open), 0xFF1A2024.toInt(), 0xFFC9D1D6.toInt())
        open.setOnClickListener { openApp() }
        box.addView(title)
        box.addView(text)
        box.addView(state)
        box.addView(silence)
        box.addView(open)
        setContentView(box)
        refresh()
    }

    override fun onResume() {
        super.onResume()
        main.post(tick)
    }

    override fun onPause() {
        super.onPause()
        main.removeCallbacks(tick)
    }

    private fun refresh() {
        val a = RadLogService.instance?.alarmInfo()
        if (a == null) {
            finish() // o alarme acabou
            return
        }
        title.text = a.title
        title.setTextColor(a.color)
        silence.visibility = if (a.quiet) android.view.View.GONE else android.view.View.VISIBLE
        state.text = if (a.quiet) getString(R.string.a_silenced) else ""
    }

    /** Abre o app só depois de desbloquear. */
    private fun openApp() {
        val go = {
            startActivity(Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            finish()
        }
        val km = getSystemService(KeyguardManager::class.java)
        if (km == null || !km.isKeyguardLocked) {
            go()
            return
        }
        km.requestDismissKeyguard(this, object : KeyguardManager.KeyguardDismissCallback() {
            override fun onDismissSucceeded() { go() }
        })
    }
}
