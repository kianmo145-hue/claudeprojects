package org.paydar.app

import android.content.Intent
import android.net.VpnService
import android.service.quicksettings.Tile
import android.service.quicksettings.TileService
import org.paydar.core.mobile.Mobile

/** Quick-settings tile: connect or disconnect without opening the app. */
class ToggleTileService : TileService() {
    private val onState: (Core.State) -> Unit = { refresh() }

    override fun onStartListening() {
        Core.addListener(onState)
    }

    override fun onStopListening() {
        Core.removeListener(onState)
    }

    override fun onClick() {
        if (Mobile.isConnected()) {
            PaydarVpnService.stop(this)
        } else if (VpnService.prepare(this) == null) {
            PaydarVpnService.start(this)
        } else {
            // First use: the permission dialog needs the app.
            val i = Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            if (android.os.Build.VERSION.SDK_INT >= 34) {
                startActivityAndCollapse(
                    android.app.PendingIntent.getActivity(this, 0, i, android.app.PendingIntent.FLAG_IMMUTABLE),
                )
            } else {
                @Suppress("DEPRECATION")
                startActivityAndCollapse(i)
            }
        }
    }

    private fun refresh() {
        val t = qsTile ?: return
        val s = Core.state
        t.state = if (s.state == "disconnected") Tile.STATE_INACTIVE else Tile.STATE_ACTIVE
        if (android.os.Build.VERSION.SDK_INT >= 29) t.subtitle = s.server.ifEmpty { null }
        t.updateTile()
    }
}
