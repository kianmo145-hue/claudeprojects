package org.braid.app

import android.content.Context

/** Speeds in Mbit/s and sizes in MB/GB, in the app's language (Persian digits in Persian). */
object Units {
    fun mbit(bytesPerSec: Double): String {
        val m = bytesPerSec * 8 / 1e6
        return if (m < 9.95) "%.1f".format(m) else "%.0f".format(m)
    }

    fun rate(ctx: Context, bytesPerSec: Double): String = ctx.getString(R.string.unit_mbps, mbit(bytesPerSec))

    fun size(ctx: Context, bytes: Double): String = when {
        bytes >= 1e9 -> ctx.getString(R.string.unit_gb, "%.2f".format(bytes / 1e9))
        bytes >= 1e6 -> ctx.getString(R.string.unit_mb, "%.1f".format(bytes / 1e6))
        else -> ctx.getString(R.string.unit_kb, "%.0f".format(bytes / 1e3))
    }
}

/** Network names come from [NetworkTracker] in English ("Wi-Fi", "Mobile data 2"); show them translated. */
object Names {
    fun network(ctx: Context, name: String): String = when {
        name.startsWith("Wi-Fi") -> ctx.getString(R.string.net_wifi) + name.removePrefix("Wi-Fi")
        name.startsWith("Mobile data") -> ctx.getString(R.string.net_cell) + name.removePrefix("Mobile data")
        else -> name
    }

    fun status(ctx: Context, status: String): String = ctx.getString(
        when (status) {
            "up" -> R.string.status_up
            "checking" -> R.string.status_checking
            else -> R.string.status_down
        },
    )
}
