package app.radlog

import android.annotation.SuppressLint
import android.bluetooth.BluetoothDevice
import android.bluetooth.BluetoothGatt
import android.bluetooth.BluetoothGattCallback
import android.bluetooth.BluetoothGattCharacteristic
import android.bluetooth.BluetoothGattDescriptor
import android.bluetooth.BluetoothManager
import android.bluetooth.BluetoothProfile
import android.bluetooth.le.ScanCallback
import android.bluetooth.le.ScanResult
import android.bluetooth.le.ScanSettings
import android.content.Context
import android.os.Build
import android.util.Log
import app.radlog.go.mobile.Mobile
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Semaphore
import java.util.concurrent.TimeUnit

/**
 * Bluetooth do RC102. O Go manda os pedidos em pedaços de 18 bytes (write)
 * e recebe as respostas pelas notificações (Mobile.bleNotify). O Android só
 * deixa uma operação do GATT por vez, então cada escrita espera a anterior.
 *
 * Todas as funções públicas são chamadas pelo Go (fora da thread principal) e
 * podem bloquear.
 */
@SuppressLint("MissingPermission") // as permissões são pedidas antes, na tela
class Ble(private val ctx: Context) {
    companion object {
        private const val TAG = "RadLog.ble"
        val WRITE: UUID = UUID.fromString("e63215e6-7003-49d8-96b0-b024798fb901")
        val NOTIFY: UUID = UUID.fromString("e63215e7-7003-49d8-96b0-b024798fb901")
        val CCCD: UUID = UUID.fromString("00002902-0000-1000-8000-00805f9b34fb")
    }

    private val manager = ctx.getSystemService(BluetoothManager::class.java)
    private val lock = Any()
    private var gatt: BluetoothGatt? = null
    private var wr: BluetoothGattCharacteristic? = null
    private var ready: CountDownLatch? = null
    private var readyErr: String? = null
    private val writeDone = Semaphore(0)
    @Volatile private var writeStatus = 0
    @Volatile private var closedAt = 0L // quando a última conexão foi fechada
    @Volatile private var lastRssi = 0 // último sinal lido do RC102 (dBm)
    @Volatile private var rssiAt = 0L
    private var rssiAsked = 0L

    /** Liga? (o Bluetooth do celular pode estar desligado) */
    fun enabled(): Boolean = manager?.adapter?.isEnabled == true

    fun scan(ms: Long): String {
        val ad = manager?.adapter ?: throw Exception("este celular não tem Bluetooth")
        if (!ad.isEnabled) throw Exception("o Bluetooth do celular está desligado")
        val sc = ad.bluetoothLeScanner ?: throw Exception("o Bluetooth do celular está desligado")
        val found = LinkedHashMap<String, JSONObject>()
        val cb = object : ScanCallback() {
            override fun onScanResult(type: Int, r: ScanResult) {
                val name = r.scanRecord?.deviceName ?: r.device.name ?: return
                synchronized(found) {
                    found[r.device.address] = JSONObject()
                        .put("address", r.device.address)
                        .put("name", name)
                        .put("rssi", r.rssi)
                        .put("paired", r.device.bondState == BluetoothDevice.BOND_BONDED)
                }
            }
        }
        val st = ScanSettings.Builder().setScanMode(ScanSettings.SCAN_MODE_LOW_LATENCY).build()
        sc.startScan(null, st, cb)
        try {
            Thread.sleep(ms.coerceIn(1000, 20000))
        } finally {
            try { sc.stopScan(cb) } catch (_: Exception) {}
        }
        val out = JSONArray()
        synchronized(found) { found.values.forEach { out.put(it) } }
        return out.toString()
    }

    private val callback = object : BluetoothGattCallback() {
        override fun onConnectionStateChange(g: BluetoothGatt, status: Int, state: Int) {
            if (state == BluetoothProfile.STATE_CONNECTED && status == BluetoothGatt.GATT_SUCCESS) {
                // conexão rápida e pacotes maiores: as respostas grandes do
                // RC102 (espectro) chegavam devagar demais com os 20 bytes
                // padrão e estouravam o tempo, e o app reconectava
                g.requestConnectionPriority(BluetoothGatt.CONNECTION_PRIORITY_HIGH)
                if (!g.requestMtu(247) && !g.discoverServices()) fail(g, "não consegui ler os serviços do aparelho")
                return
            }
            if (state == BluetoothProfile.STATE_DISCONNECTED) {
                val wasReady: Boolean
                synchronized(lock) {
                    wasReady = gatt === g && ready == null
                    if (gatt === g) { gatt = null; wr = null }
                }
                g.close()
                closedAt = android.os.SystemClock.elapsedRealtime()
                fail(g, "o aparelho não aceitou a conexão (status $status)")
                writeStatus = -1
                writeDone.release()
                if (wasReady) Mobile.bleLost()
            }
        }

        override fun onMtuChanged(g: BluetoothGatt, mtu: Int, status: Int) {
            Log.i(TAG, "MTU $mtu (status $status)")
            if (!g.discoverServices()) fail(g, "não consegui ler os serviços do aparelho")
        }

        override fun onServicesDiscovered(g: BluetoothGatt, status: Int) {
            var w: BluetoothGattCharacteristic? = null
            var n: BluetoothGattCharacteristic? = null
            for (s in g.services) for (c in s.characteristics) {
                if (c.uuid == WRITE) w = c
                if (c.uuid == NOTIFY) n = c
            }
            if (w == null || n == null) return fail(g, "o aparelho não mostrou o serviço do RC102")
            synchronized(lock) { wr = w }
            g.setCharacteristicNotification(n, true)
            val d = n.getDescriptor(CCCD) ?: return fail(g, "o aparelho não aceita notificações")
            val v = BluetoothGattDescriptor.ENABLE_NOTIFICATION_VALUE
            val ok = if (Build.VERSION.SDK_INT >= 33) {
                g.writeDescriptor(d, v) == BluetoothGatt.GATT_SUCCESS
            } else {
                @Suppress("DEPRECATION")
                d.value = v
                @Suppress("DEPRECATION")
                g.writeDescriptor(d)
            }
            if (!ok) fail(g, "não consegui ligar as notificações")
        }

        override fun onDescriptorWrite(g: BluetoothGatt, d: BluetoothGattDescriptor, status: Int) {
            if (status != BluetoothGatt.GATT_SUCCESS) return fail(g, "não consegui ligar as notificações (status $status)")
            val l: CountDownLatch?
            synchronized(lock) { l = ready; ready = null }
            l?.countDown()
        }

        override fun onReadRemoteRssi(g: BluetoothGatt, rssi: Int, status: Int) {
            if (status == BluetoothGatt.GATT_SUCCESS) { lastRssi = rssi; rssiAt = android.os.SystemClock.elapsedRealtime() }
        }

        override fun onCharacteristicWrite(g: BluetoothGatt, c: BluetoothGattCharacteristic, status: Int) {
            writeStatus = status
            writeDone.release()
        }

        override fun onCharacteristicChanged(g: BluetoothGatt, c: BluetoothGattCharacteristic, value: ByteArray) {
            if (c.uuid == NOTIFY) Mobile.bleNotify(value)
        }

        @Deprecated("Android 12 e anteriores")
        override fun onCharacteristicChanged(g: BluetoothGatt, c: BluetoothGattCharacteristic) {
            @Suppress("DEPRECATION")
            if (Build.VERSION.SDK_INT < 33 && c.uuid == NOTIFY) Mobile.bleNotify(c.value)
        }
    }

    private fun fail(g: BluetoothGatt, msg: String) {
        val l: CountDownLatch?
        synchronized(lock) {
            l = ready
            if (l != null) { ready = null; readyErr = msg }
        }
        if (l != null) {
            Log.w(TAG, msg)
            try { g.disconnect() } catch (_: Exception) {}
            l.countDown()
        }
    }

    fun connect(addr: String) {
        val ad = manager?.adapter ?: throw Exception("este celular não tem Bluetooth")
        if (!ad.isEnabled) throw Exception("o Bluetooth do celular está desligado")
        // a conexão anterior acabou de cair: o Android precisa de um instante
        // para soltar o rádio (sem isso vinha status 133 ou 25 s sem resposta)
        close()
        val since = android.os.SystemClock.elapsedRealtime() - closedAt
        if (since in 0 until 1500) Thread.sleep(1500 - since)
        val dev = try { ad.getRemoteDevice(addr) } catch (e: IllegalArgumentException) {
            throw Exception("endereço Bluetooth inválido: $addr")
        }
        val l = CountDownLatch(1)
        val g: BluetoothGatt
        synchronized(lock) {
            ready = l
            readyErr = null
            g = dev.connectGatt(ctx, false, callback, BluetoothDevice.TRANSPORT_LE)
                ?: throw Exception("o Android não abriu a conexão")
            gatt = g
        }
        if (!l.await(15, TimeUnit.SECONDS)) {
            synchronized(lock) { ready = null }
            close()
            throw Exception("o RC102 não respondeu (longe, desligado ou com outro aparelho)")
        }
        readyErr?.let { close(); throw Exception(it) }
        while (writeDone.tryAcquire()) { /* sobras */ }
    }

    fun write(chunk: ByteArray) {
        val g: BluetoothGatt
        val c: BluetoothGattCharacteristic
        synchronized(lock) {
            g = gatt ?: throw Exception("conexão perdida")
            c = wr ?: throw Exception("conexão perdida")
        }
        // o sinal do RC102 a cada 20 s, para o registro dizer se a queda foi rádio fraco
        val now = android.os.SystemClock.elapsedRealtime()
        if (now - rssiAsked > 20_000) { rssiAsked = now; try { g.readRemoteRssi() } catch (_: Exception) {} }
        val t = BluetoothGattCharacteristic.WRITE_TYPE_NO_RESPONSE
        var tries = 0
        while (true) {
            val ok = if (Build.VERSION.SDK_INT >= 33) {
                g.writeCharacteristic(c, chunk, t) == BluetoothGatt.GATT_SUCCESS
            } else {
                @Suppress("DEPRECATION")
                c.writeType = t
                @Suppress("DEPRECATION")
                c.value = chunk
                @Suppress("DEPRECATION")
                g.writeCharacteristic(c)
            }
            if (ok) break
            // fila do GATT ocupada: espera um pouco e tenta de novo
            if (++tries > 50) throw Exception("o Android não aceitou o envio")
            Thread.sleep(10)
        }
        if (!writeDone.tryAcquire(3, TimeUnit.SECONDS)) throw Exception("o envio não terminou")
        if (writeStatus != BluetoothGatt.GATT_SUCCESS) throw Exception("envio recusado (status $writeStatus)")
    }

    // último sinal lido (dBm), se tiver menos de 1 min; 0 = não sabe
    fun rssi(): Int = if (android.os.SystemClock.elapsedRealtime() - rssiAt < 60_000) lastRssi else 0

    fun close() {
        val g: BluetoothGatt?
        synchronized(lock) { g = gatt; gatt = null; wr = null }
        if (g != null) {
            try { g.disconnect() } catch (_: Exception) {}
            try { g.close() } catch (_: Exception) {}
            closedAt = android.os.SystemClock.elapsedRealtime()
        }
    }
}
