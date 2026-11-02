package org.braid.app

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import android.widget.Toast
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.appcompat.app.AppCompatDelegate
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.core.os.LocaleListCompat
import org.braid.app.ui.App
import org.braid.app.ui.BoosterScreen
import org.braid.app.ui.MergeScreen
import org.braid.app.ui.PairDialog
import org.braid.app.ui.BraidTheme
import org.braid.app.ui.SettingsScreen
import org.braid.app.ui.Tab

class MainActivity : AppCompatActivity() {

    private val vpnConsent = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
        if (result.resultCode == RESULT_OK) {
            MergeVpnService.start(this)
        } else {
            Toast.makeText(this, R.string.vpn_denied, Toast.LENGTH_LONG).show()
        }
    }

    private val notifPermission = registerForActivityResult(ActivityResultContracts.RequestPermission()) { }

    /** The tab asked for by a notification ("booster") when the activity is (re)opened. */
    private var requestedTab by mutableStateOf<Tab?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        applyTheme(Prefs.theme(this))
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        Engine.loadSettings(this)
        Booster.loadPcs(this)
        requestedTab = tabFrom(intent)
        setContent {
            var theme by rememberSaveable { mutableStateOf(Prefs.theme(this)) }
            var tab by rememberSaveable { mutableStateOf(requestedTab ?: if (Booster.running) Tab.BOOSTER else Tab.MERGE) }
            val req = requestedTab
            LaunchedEffect(req) {
                if (req != null) {
                    tab = req
                    requestedTab = null
                }
            }
            BraidTheme {
                val phase by Engine.phase.collectAsState()
                val stats by Engine.stats.collectAsState()
                val history by Engine.history.collectAsState()
                val even by Engine.evenSplit.collectAsState()
                val bPhase by Booster.phase.collectAsState()
                val links by Booster.links.collectAsState()
                val clients by Booster.clients.collectAsState()
                val pcs by Booster.pcs.collectAsState()
                val bHistory by Booster.history.collectAsState()
                val pair by Booster.pairRequest.collectAsState()

                App(tab = tab, onTab = { tab = it }) { t ->
                    when (t) {
                        Tab.MERGE -> MergeScreen(phase, stats, history, onToggle = ::toggleMerge)
                        Tab.BOOSTER -> BoosterScreen(
                            bPhase, links, clients, pcs, bHistory,
                            onToggle = ::toggleBoost,
                            onForget = { Booster.forget(this, it) },
                        )
                        Tab.SETTINGS -> SettingsScreen(
                            lang = currentLang(),
                            theme = theme,
                            evenSplit = even,
                            onLang = ::setLang,
                            onTheme = {
                                Prefs.setTheme(this, it)
                                theme = it
                                applyTheme(it)
                            },
                            onEvenSplit = { Engine.setEvenSplit(this, it) },
                        )
                    }
                }
                pair?.let { req -> PairDialog(req) { ok -> BoostService.answer(ok) } }
            }
        }
        if (Build.VERSION.SDK_INT >= 33 &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            notifPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
        // Debug builds only: lets a CI job switch merging on without tapping.
        if (BuildConfig.DEBUG && intent.getBooleanExtra("autostart", false) && Engine.phase.value == Engine.Phase.OFF) {
            toggleMerge()
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        tabFrom(intent)?.let { requestedTab = it }
    }

    private fun tabFrom(i: Intent?): Tab? = if (i?.getStringExtra("tab") == "booster") Tab.BOOSTER else null

    private fun toggleMerge() {
        if (Engine.phase.value != Engine.Phase.OFF) {
            MergeVpnService.stop(this)
            return
        }
        val consent = VpnService.prepare(this)
        if (consent != null) vpnConsent.launch(consent) else MergeVpnService.start(this)
    }

    private fun toggleBoost() {
        if (Booster.phase.value != Booster.Phase.OFF) BoostService.stop(this) else BoostService.start(this)
    }

    private fun currentLang(): String {
        val l = AppCompatDelegate.getApplicationLocales()
        return if (l.isEmpty) "system" else l[0]?.language ?: "system"
    }

    /** Per-app language (Android 13+ also shows it in the system's app settings). */
    private fun setLang(lang: String) {
        val list = if (lang == "system") LocaleListCompat.getEmptyLocaleList() else LocaleListCompat.forLanguageTags(lang)
        AppCompatDelegate.setApplicationLocales(list)
    }

    private fun applyTheme(t: String) {
        AppCompatDelegate.setDefaultNightMode(
            when (t) {
                "light" -> AppCompatDelegate.MODE_NIGHT_NO
                "dark" -> AppCompatDelegate.MODE_NIGHT_YES
                else -> AppCompatDelegate.MODE_NIGHT_FOLLOW_SYSTEM
            },
        )
    }
}
