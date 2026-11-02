package org.peyvand.app

import android.content.Context
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/**
 * Merge mode's state, shared by the VPN service, the UI and the Quick Settings tile. The service
 * samples the Go engine once a second; the UI only observes.
 */
object Engine {
    enum class Phase { OFF, STARTING, ON }

    /** Seconds of throughput history kept for the live chart. */
    const val HISTORY = 60

    private val _phase = MutableStateFlow(Phase.OFF)
    val phase: StateFlow<Phase> = _phase.asStateFlow()

    private val _stats = MutableStateFlow<List<Stats>>(emptyList())
    val stats: StateFlow<List<Stats>> = _stats.asStateFlow()

    /** Per network: total (down + up) bytes/s for the last [HISTORY] seconds, oldest first. */
    private val _history = MutableStateFlow<Map<String, List<Float>>>(emptyMap())
    val history: StateFlow<Map<String, List<Float>>> = _history.asStateFlow()

    private val _evenSplit = MutableStateFlow(false)
    val evenSplit: StateFlow<Boolean> = _evenSplit.asStateFlow()

    val running: Boolean get() = _phase.value == Phase.ON

    internal fun setPhase(p: Phase) {
        _phase.value = p
        if (p == Phase.OFF) {
            _stats.value = emptyList()
            _history.value = emptyMap()
        }
    }

    /** Reads the engine's counters. Called by the service every second while it runs. */
    internal fun sample() {
        val now = Stats.parse(peyvand.Peyvand.stats()).filter { it.present }
        _stats.value = now
        val old = _history.value
        _history.value = now.associate { s ->
            val prev = old[s.name] ?: List(HISTORY) { 0f }
            s.name to (prev + (s.rateIn + s.rateOut).toFloat()).takeLast(HISTORY)
        }
    }

    fun loadSettings(ctx: Context) {
        _evenSplit.value = Prefs.evenSplit(ctx)
        applySettings()
    }

    fun setEvenSplit(ctx: Context, on: Boolean) {
        Prefs.setEvenSplit(ctx, on)
        _evenSplit.value = on
        applySettings()
    }

    private fun applySettings() {
        runCatching { peyvand.Peyvand.setAutoWeights(!_evenSplit.value) }
    }
}
