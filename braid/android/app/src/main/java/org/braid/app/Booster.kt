package org.braid.app

import android.content.Context
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import org.json.JSONArray

/**
 * Booster mode's state: this phone shares its mobile data with paired computers on the same
 * Wi-Fi. [BoostService] drives it; the UI observes.
 */
object Booster {
    enum class Phase { OFF, ON }

    data class Client(
        val id: String,
        val name: String,
        val active: Long,
        val bytesIn: Long,
        val bytesOut: Long,
        val lastSeen: Long,
        /** bytes/s over the last second, both directions. */
        val rateIn: Double = 0.0,
        val rateOut: Double = 0.0,
    )

    data class PairRequest(val pcName: String, val code: String)

    /** What the phone has right now: both are needed to share. */
    data class Links(val wifi: Boolean = false, val cell: Boolean = false, val wifiIp: String = "")

    private val _phase = MutableStateFlow(Phase.OFF)
    val phase: StateFlow<Phase> = _phase.asStateFlow()

    private val _clients = MutableStateFlow<List<Client>>(emptyList())
    val clients: StateFlow<List<Client>> = _clients.asStateFlow()

    private val _history = MutableStateFlow(List(Engine.HISTORY) { 0f })
    val history: StateFlow<List<Float>> = _history.asStateFlow()

    private val _pair = MutableStateFlow<PairRequest?>(null)
    val pairRequest: StateFlow<PairRequest?> = _pair.asStateFlow()

    private val _pcs = MutableStateFlow<List<Prefs.Pc>>(emptyList())
    val pcs: StateFlow<List<Prefs.Pc>> = _pcs.asStateFlow()

    private val _links = MutableStateFlow(Links())
    val links: StateFlow<Links> = _links.asStateFlow()

    val running: Boolean get() = _phase.value == Phase.ON

    internal fun setPhase(p: Phase) {
        _phase.value = p
        if (p == Phase.OFF) {
            _clients.value = emptyList()
            _history.value = List(Engine.HISTORY) { 0f }
            _pair.value = null
        }
    }

    internal fun setLinks(l: Links) {
        _links.value = l
    }

    internal fun setPairRequest(r: PairRequest?) {
        _pair.value = r
    }

    fun loadPcs(ctx: Context) {
        _pcs.value = Prefs.pcs(ctx)
    }

    /** Forgets a computer here and in the running service. */
    fun forget(ctx: Context, id: String) {
        Prefs.removePc(ctx, id)
        loadPcs(ctx)
        runCatching { braid.Braid.boostSetPeers(Prefs.pcsJson(_pcs.value)) }
    }

    /** Reads the per-computer counters. Called by the service every second. */
    internal fun sample() {
        val arr = runCatching { JSONArray(braid.Braid.boostStats()) }.getOrNull() ?: return
        val old = _clients.value.associateBy { it.id }
        val names = _pcs.value.associate { it.id to it.name }
        val now = (0 until arr.length()).map {
            val o = arr.getJSONObject(it)
            val id = o.optString("id")
            val bIn = o.optLong("bytesIn")
            val bOut = o.optLong("bytesOut")
            val prev = old[id]
            Client(
                id = id,
                name = o.optString("name").ifEmpty { names[id] ?: id.take(6) },
                active = o.optLong("active"),
                bytesIn = bIn,
                bytesOut = bOut,
                lastSeen = o.optLong("lastSeen"),
                rateIn = if (prev != null) (bIn - prev.bytesIn).coerceAtLeast(0).toDouble() else 0.0,
                rateOut = if (prev != null) (bOut - prev.bytesOut).coerceAtLeast(0).toDouble() else 0.0,
            )
        }
        _clients.value = now
        _history.value = (_history.value + now.sumOf { it.rateIn + it.rateOut }.toFloat()).takeLast(Engine.HISTORY)
    }
}
