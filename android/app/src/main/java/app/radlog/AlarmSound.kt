package app.radlog

import android.media.AudioAttributes
import android.media.AudioFormat
import android.media.AudioTrack
import kotlin.math.exp
import kotlin.math.ln

/**
 * Os sons do alarme do desktop (frontend/dist/app.js, startSound), feitos
 * aqui com AudioTrack para tocar com a tela apagada e o app em segundo plano:
 *  - sirene: duas dentes-de-serra levemente desafinadas varrendo 600–1500 Hz
 *    (1 s por volta, 0,5 s no "rápido");
 *  - bipes: 4 bipes quadrados de 2200 Hz a cada 0,9 s (0,65 s no rápido);
 *  - perigo: duas quadradas em trítono subindo de 900 a 2600 Hz 4× por
 *    segundo, picotadas no fim de cada subida.
 * Toca no volume de alarme do Android (USAGE_ALARM).
 */
object AlarmSound {
    private const val RATE = 44100
    @Volatile private var mode: String? = null
    @Volatile private var fast = false
    private var thread: Thread? = null

    val playing: String? get() = mode?.let { if (fast) "$it+" else it }

    @Synchronized
    fun play(m: String, f: Boolean) {
        if (m == "mudo") { stop(); return }
        if (mode == m && fast == f && thread?.isAlive == true) return
        mode = m
        fast = f
        if (thread?.isAlive == true) return
        thread = Thread({ loop() }, "radlog-alarme").also { it.start() }
    }

    @Synchronized
    fun stop() {
        mode = null
        thread?.interrupt()
        thread = null
    }

    private fun loop() {
        val attrs = AudioAttributes.Builder()
            .setUsage(AudioAttributes.USAGE_ALARM)
            .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
            .build()
        val fmt = AudioFormat.Builder().setSampleRate(RATE)
            .setEncoding(AudioFormat.ENCODING_PCM_16BIT)
            .setChannelMask(AudioFormat.CHANNEL_OUT_MONO).build()
        val min = AudioTrack.getMinBufferSize(RATE, AudioFormat.CHANNEL_OUT_MONO, AudioFormat.ENCODING_PCM_16BIT)
        val track = AudioTrack.Builder().setAudioAttributes(attrs).setAudioFormat(fmt)
            .setBufferSizeInBytes(maxOf(min, RATE / 5 * 2)).setTransferMode(AudioTrack.MODE_STREAM).build()
        val buf = ShortArray(RATE / 20) // 50 ms por vez
        var t = 0L
        val ph = DoubleArray(2)
        try {
            track.play()
            while (!Thread.currentThread().isInterrupted) {
                val m = mode ?: break
                val f = fast
                for (i in buf.indices) {
                    val s = (t + i).toDouble() / RATE
                    buf[i] = (sample(m, f, s, ph) * 32767).toInt().coerceIn(-32767, 32767).toShort()
                }
                t += buf.size
                track.write(buf, 0, buf.size)
            }
        } catch (_: Exception) {
        } finally {
            try { track.stop() } catch (_: Exception) {}
            track.release()
        }
    }

    private fun saw(p: Double) = 2 * (p - Math.floor(p + 0.5))
    private fun sq(p: Double) = if (p - Math.floor(p) < 0.5) 1.0 else -1.0

    private fun sample(m: String, f: Boolean, s: Double, ph: DoubleArray): Double {
        val dt = 1.0 / RATE
        when (m) {
            "perigo" -> {
                val period = 0.25
                val x = (s % period) / period
                val fr = 900 * exp(ln(2600.0 / 900) * minOf(x / 0.9, 1.0))
                ph[0] += fr * dt
                ph[1] += fr * Math.sqrt(2.0) * dt
                val g = if (x < 0.85) 0.42 else 0.42 - (0.40 * (x - 0.85) / 0.13).coerceAtMost(0.40)
                return g * 0.5 * (sq(ph[0]) + sq(ph[1]))
            }
            "bipes" -> {
                val every = if (f) 0.65 else 0.9
                val x = s % every
                val k = (x / 0.14).toInt()
                val y = x - k * 0.14
                if (k > 3 || y > 0.1) return 0.0
                val env = when {
                    y < 0.005 -> y / 0.005
                    y < 0.08 -> 1.0
                    else -> 1 - (y - 0.08) / 0.02
                }
                return 0.4 * env * sq(2200 * s)
            }
            else -> { // sirene
                val period = if (f) 0.5 else 1.0
                val x = (s % period) / period
                val fr = if (x < 0.5) 600 + 900 * (x / 0.5) else 1500 - 900 * ((x - 0.5) / 0.5)
                ph[0] += fr * dt
                ph[1] += fr * Math.pow(2.0, 70.0 / 1200) * dt
                val g = 0.28 * minOf(s / 0.05, 1.0)
                return g * 0.5 * (saw(ph[0]) + saw(ph[1]))
            }
        }
    }

}
