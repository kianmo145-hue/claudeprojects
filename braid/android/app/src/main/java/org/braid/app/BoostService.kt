package org.braid.app

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.util.Log

/**
 * Booster mode: shares this phone's mobile data with paired computers on the same Wi-Fi.
 *
 * Wi-Fi and mobile data are both kept up (see [NetworkTracker]). The Go side listens on the Wi-Fi
 * address for paired computers, announces the phone on the Wi-Fi every two seconds, and connects
 * every tunnel out over mobile data only.
 */
class BoostService : Service(), braid.BoostListener {

    companion object {
        const val ACTION_START = "org.braid.app.BOOST_START"
        const val ACTION_STOP = "org.braid.app.BOOST_STOP"
        const val ACTION_APPROVE = "org.braid.app.PAIR_APPROVE"
        const val ACTION_DECLINE = "org.braid.app.PAIR_DECLINE"
        private const val TAG = "BraidBoost"
        private const val CHANNEL = "booster"
        private const val CHANNEL_PAIR = "pairing"
        private const val NOTIF_ID = 2
        private const val NOTIF_PAIR = 3

        fun start(ctx: Context) {
            ctx.startForegroundService(Intent(ctx, BoostService::class.java).setAction(ACTION_START))
        }

        fun stop(ctx: Context) {
            ctx.startService(Intent(ctx, BoostService::class.java).setAction(ACTION_STOP))
        }

        /** Answers the pending pairing request (from the in-app dialog). */
        fun answer(approve: Boolean) {
            runCatching { braid.Braid.boostAnswer(approve) }
        }
    }

    private var tracker: NetworkTracker? = null
    private var ticks = 0
    private val main = Handler(Looper.getMainLooper())
    private val refresh = object : Runnable {
        override fun run() {
            Booster.sample()
            if (ticks++ % 2 == 0) updateNotification()
            main.postDelayed(this, 1000)
        }
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> {
                shutdown()
                return START_NOT_STICKY
            }
            ACTION_APPROVE -> {
                answer(true)
                return START_NOT_STICKY
            }
            ACTION_DECLINE -> {
                answer(false)
                return START_NOT_STICKY
            }
        }
        goForeground()
        if (Booster.phase.value == Booster.Phase.OFF) begin()
        return START_STICKY
    }

    private fun begin() {
        // Merge mode and booster mode are one or the other: both would fight over mobile data.
        if (Engine.phase.value != Engine.Phase.OFF) MergeVpnService.stop(this)
        Booster.loadPcs(this)
        val t = NetworkTracker(this) { nets -> main.post { onNetworks(nets) } }
        tracker = t
        t.start()
        try {
            braid.Braid.boostSetPeers(Prefs.pcsJson(Booster.pcs.value))
            braid.Braid.boostStart(Prefs.phoneId(this), Prefs.phoneName(this), this)
        } catch (e: Exception) {
            Log.e(TAG, "boost start failed", e)
            shutdown()
            return
        }
        Booster.setPhase(Booster.Phase.ON)
        onNetworks(t.current())
        main.post(refresh)
    }

    private fun onNetworks(nets: List<NetworkTracker.Tracked>) {
        if (Booster.phase.value == Booster.Phase.OFF && tracker == null) return
        val wifi = nets.firstOrNull { it.wifi && it.ipv4 != null }
        val cell = nets.firstOrNull { !it.wifi && (it.v4 || it.v6) }
        braid.Braid.boostSetNetworks(
            cell?.network?.networkHandle?.toString() ?: "",
            wifi?.network?.networkHandle?.toString() ?: "",
            wifi?.ipv4 ?: "",
            (wifi?.prefix ?: 0).toLong(),
        )
        Booster.setLinks(Booster.Links(wifi = wifi != null, cell = cell != null, wifiIp = wifi?.ipv4 ?: ""))
        updateNotification()
    }

    private fun shutdown() {
        main.removeCallbacks(refresh)
        runCatching { braid.Braid.boostStop() }
        Booster.setPhase(Booster.Phase.OFF)
        Booster.setLinks(Booster.Links())
        tracker?.stop()
        tracker = null
        getSystemService(NotificationManager::class.java).cancel(NOTIF_PAIR)
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    override fun onDestroy() {
        if (Booster.phase.value != Booster.Phase.OFF) shutdown()
        super.onDestroy()
    }

    // --- braid.BoostListener (called from Go threads) ---

    override fun onPairRequest(pcName: String, code: String) {
        main.post {
            Booster.setPairRequest(Booster.PairRequest(pcName, code))
            showPairNotification(pcName, code)
        }
    }

    override fun onPaired(pcID: String, pcName: String, keyHex: String) {
        main.post {
            Prefs.savePc(this, Prefs.Pc(pcID, pcName, keyHex))
            Booster.loadPcs(this)
        }
    }

    override fun onPairEnded() {
        main.post {
            Booster.setPairRequest(null)
            getSystemService(NotificationManager::class.java).cancel(NOTIF_PAIR)
        }
    }

    // --- notifications ---

    private fun channels() {
        val nm = getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(
            NotificationChannel(CHANNEL, getString(R.string.boost_channel), NotificationManager.IMPORTANCE_LOW),
        )
        nm.createNotificationChannel(
            NotificationChannel(CHANNEL_PAIR, getString(R.string.pair_channel), NotificationManager.IMPORTANCE_HIGH),
        )
    }

    private fun goForeground() {
        channels()
        val n = buildNotification(getString(R.string.boost_waiting))
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            startForeground(NOTIF_ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        } else {
            startForeground(NOTIF_ID, n)
        }
    }

    private fun openApp(): PendingIntent = PendingIntent.getActivity(
        this, 10, Intent(this, MainActivity::class.java).putExtra("tab", "booster"),
        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
    )

    private fun action(act: String, code: Int): PendingIntent = PendingIntent.getService(
        this, code, Intent(this, BoostService::class.java).setAction(act), PendingIntent.FLAG_IMMUTABLE,
    )

    private fun buildNotification(text: String): Notification =
        Notification.Builder(this, CHANNEL)
            .setSmallIcon(R.drawable.ic_phone_link)
            .setContentTitle(getString(R.string.boost_notif_title))
            .setContentText(text)
            .setStyle(Notification.BigTextStyle().bigText(text))
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setContentIntent(openApp())
            .addAction(Notification.Action.Builder(null, getString(R.string.stop), action(ACTION_STOP, 11)).build())
            .build()

    private fun updateNotification() {
        if (Booster.phase.value != Booster.Phase.ON) return
        val l = Booster.links.value
        val clients = Booster.clients.value.filter { it.active > 0 || it.rateIn + it.rateOut > 0 }
        val text = when {
            !l.wifi -> getString(R.string.boost_need_wifi)
            !l.cell -> getString(R.string.boost_need_cell)
            clients.isEmpty() -> getString(R.string.boost_ready)
            else -> clients.joinToString("\n") {
                getString(R.string.boost_client_line, it.name, Units.rate(this, it.rateIn + it.rateOut))
            }
        }
        getSystemService(NotificationManager::class.java).notify(NOTIF_ID, buildNotification(text))
    }

    private fun showPairNotification(pcName: String, code: String) {
        channels()
        val pretty = code.take(3) + " " + code.drop(3)
        val n = Notification.Builder(this, CHANNEL_PAIR)
            .setSmallIcon(R.drawable.ic_phone_link)
            .setContentTitle(getString(R.string.pair_title, pcName))
            .setContentText(getString(R.string.pair_notif_text, pretty))
            .setStyle(Notification.BigTextStyle().bigText(getString(R.string.pair_notif_text, pretty)))
            .setCategory(Notification.CATEGORY_CALL)
            .setAutoCancel(true)
            .setContentIntent(openApp())
            .addAction(Notification.Action.Builder(null, getString(R.string.pair_decline), action(ACTION_DECLINE, 12)).build())
            .addAction(Notification.Action.Builder(null, getString(R.string.pair_approve), action(ACTION_APPROVE, 13)).build())
            .build()
        getSystemService(NotificationManager::class.java).notify(NOTIF_PAIR, n)
    }
}
