package org.paydar.app

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.ConnectivityManager
import android.net.LinkProperties
import android.net.Network
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.service.quicksettings.TileService
import android.util.Log
import org.paydar.core.mobile.Mobile
import java.net.Inet4Address
import kotlin.concurrent.thread

/**
 * Owns the VPN: creates the TUN, hands it to Go, follows the default network, and shows the
 * status notification. The app's own traffic is excluded from the VPN, so sing-box reaches the
 * servers directly.
 */
class PaydarVpnService : VpnService() {

    companion object {
        private const val TAG = "Paydar"
        private const val ACTION_START = "org.paydar.app.START"
        private const val ACTION_STOP = "org.paydar.app.STOP"
        private const val CHANNEL = "status"
        private const val NOTIF_ID = 1
        private const val MTU = 9000

        fun start(ctx: Context) {
            ctx.startForegroundService(Intent(ctx, PaydarVpnService::class.java).setAction(ACTION_START))
        }

        fun stop(ctx: Context) {
            ctx.startService(Intent(ctx, PaydarVpnService::class.java).setAction(ACTION_STOP))
        }
    }

    private var tun: ParcelFileDescriptor? = null
    private var netCallback: ConnectivityManager.NetworkCallback? = null
    @Volatile private var stopping = false
    private val onState: (Core.State) -> Unit = { updateNotification(it) }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP) {
            shutdown()
            return START_NOT_STICKY
        }
        // ACTION_START, or the system starting us for always-on VPN.
        goForeground(Core.State("connecting", "", false))
        if (tun == null) begin()
        return START_STICKY
    }

    private fun begin() {
        stopping = false
        Core.addListener(onState)
        val cm = getSystemService(ConnectivityManager::class.java)
        val dns = dnsOf(cm.getLinkProperties(cm.activeNetwork))
        val builder = Builder()
            .setSession(getString(R.string.app_name))
            .setMtu(MTU)
            .addAddress("172.19.0.1", 30)
            .addRoute("0.0.0.0", 0)
            // Queries to this address are answered by sing-box through the tunnel.
            .addDnsServer("172.19.0.2")
            .addDisallowedApplication(packageName)
            .setConfigureIntent(
                PendingIntent.getActivity(this, 0, Intent(this, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE),
            )
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) builder.setMetered(false)
        // IPv6 is captured too (and blocked unless enabled) so it cannot leak around the tunnel.
        runCatching {
            builder.addAddress("fdfe:dcba:9876::1", 126)
            builder.addRoute("::", 0)
        }
        val pfd = try {
            builder.establish()
        } catch (e: Exception) {
            Log.e(TAG, "establish", e)
            null
        }
        if (pfd == null) {
            shutdown()
            return
        }
        tun = pfd
        val fd = pfd.detachFd()
        thread(name = "paydar-start") {
            try {
                Mobile.startTunnel(fd.toLong(), MTU.toLong(), dns)
                watchNetwork(cm)
                TileService.requestListeningState(this, ComponentName(this, ToggleTileService::class.java))
            } catch (e: Exception) {
                Log.e(TAG, "start tunnel", e)
                // Go did not take the descriptor; close it ourselves.
                runCatching { ParcelFileDescriptor.adoptFd(fd).close() }
                tun = null
                shutdown()
            }
        }
    }

    private fun dnsOf(lp: LinkProperties?): String =
        lp?.dnsServers.orEmpty().filterIsInstance<Inet4Address>().joinToString(",") { it.hostAddress ?: "" }

    private fun watchNetwork(cm: ConnectivityManager) {
        val cb = object : ConnectivityManager.NetworkCallback() {
            private var last: Network? = null
            private var lastDns = ""

            override fun onLinkPropertiesChanged(network: Network, lp: LinkProperties) {
                val dns = dnsOf(lp)
                if (network == last && dns == lastDns) return
                last = network
                lastDns = dns
                setUnderlyingNetworks(arrayOf(network))
                thread { Mobile.networkChanged(dns) }
            }

            override fun onLost(network: Network) {
                if (network == last) setUnderlyingNetworks(null)
            }
        }
        // We are excluded from our own VPN, so our default network is the real one.
        cm.registerDefaultNetworkCallback(cb)
        netCallback = cb
    }

    private fun shutdown() {
        if (stopping) return
        stopping = true
        Core.removeListener(onState)
        netCallback?.let { cb -> runCatching { getSystemService(ConnectivityManager::class.java).unregisterNetworkCallback(cb) } }
        netCallback = null
        thread(name = "paydar-stop") {
            Mobile.stopTunnel()
            tun = null
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
            TileService.requestListeningState(this, ComponentName(this, ToggleTileService::class.java))
        }
    }

    override fun onRevoke() {
        // Another VPN started or the user turned us off in Settings.
        shutdown()
    }

    override fun onDestroy() {
        shutdown()
        super.onDestroy()
    }

    private fun goForeground(s: Core.State) {
        val nm = getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(
            NotificationChannel(CHANNEL, getString(R.string.notif_channel), NotificationManager.IMPORTANCE_LOW),
        )
        val n = buildNotification(s)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            startForeground(NOTIF_ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_SYSTEM_EXEMPTED)
        } else {
            startForeground(NOTIF_ID, n)
        }
    }

    private fun updateNotification(s: Core.State) {
        if (stopping || s.state == "disconnected") return
        getSystemService(NotificationManager::class.java).notify(NOTIF_ID, buildNotification(s))
    }

    private fun buildNotification(s: Core.State): Notification {
        val text = when {
            s.state == "connected" && s.offline -> getString(R.string.notif_offline)
            s.state == "connected" -> getString(R.string.notif_connected, s.server)
            else -> getString(R.string.notif_connecting)
        }
        val open = PendingIntent.getActivity(this, 0, Intent(this, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE)
        val stop = PendingIntent.getService(
            this, 1, Intent(this, PaydarVpnService::class.java).setAction(ACTION_STOP), PendingIntent.FLAG_IMMUTABLE,
        )
        return Notification.Builder(this, CHANNEL)
            .setSmallIcon(R.drawable.ic_shield)
            .setContentTitle(getString(R.string.app_name))
            .setContentText(text)
            .setContentIntent(open)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .addAction(Notification.Action.Builder(null, getString(R.string.notif_disconnect), stop).build())
            .build()
    }
}
