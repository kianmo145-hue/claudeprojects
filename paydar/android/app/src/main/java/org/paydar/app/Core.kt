package org.paydar.app

import android.content.Context
import android.os.Handler
import android.os.Looper
import android.util.Log
import org.paydar.core.mobile.Mobile
import org.paydar.core.mobile.Platform
import java.lang.ref.WeakReference
import java.util.concurrent.CopyOnWriteArrayList

/**
 * The Go core (servers, guard, sing-box, UI server) lives once per process. The UI's connect and
 * disconnect buttons come back here, because only Android can open the VPN.
 */
object Core : Platform {
    private const val TAG = "Paydar"

    data class State(val state: String, val server: String, val offline: Boolean)

    private lateinit var app: Context
    private val main = Handler(Looper.getMainLooper())
    private var activity: WeakReference<MainActivity>? = null
    private val listeners = CopyOnWriteArrayList<(State) -> Unit>()

    @Volatile var state = State("disconnected", "", false)
        private set
    @Volatile var uiUrl: String? = null
        private set
    @Volatile var startError: String? = null
        private set

    fun init(ctx: Context) {
        app = ctx.applicationContext
        try {
            uiUrl = Mobile.start(app.filesDir.absolutePath, BuildConfig.VERSION_NAME, this)
        } catch (e: Exception) {
            Log.e(TAG, "core start failed", e)
            startError = e.message
        }
    }

    fun attach(a: MainActivity?) {
        activity = a?.let { WeakReference(it) }
    }

    fun addListener(l: (State) -> Unit) {
        listeners += l
        l(state)
    }

    fun removeListener(l: (State) -> Unit) {
        listeners -= l
    }

    // ---- Platform (called from Go, on Go threads) ----

    override fun requestConnect() {
        main.post {
            val a = activity?.get()
            if (a != null && !a.isFinishing) {
                a.connectWithPermission()
            } else if (android.net.VpnService.prepare(app) == null) {
                PaydarVpnService.start(app)
            }
        }
    }

    override fun requestDisconnect() {
        main.post { PaydarVpnService.stop(app) }
    }

    override fun onStatus(state: String, server: String, offline: Boolean) {
        val s = State(state, server, offline)
        this.state = s
        main.post { listeners.forEach { it(s) } }
    }
}
