package org.peyvand.app

import android.content.Context
import android.os.Build
import android.provider.Settings
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID

/**
 * Everything the app remembers. Paired computers (and their keys) live in app-private storage;
 * backups are off in the manifest, so the keys never leave the phone.
 */
object Prefs {
    private fun sp(ctx: Context) = ctx.getSharedPreferences("settings", Context.MODE_PRIVATE)

    /** A computer paired for booster mode. [key] is the 32-byte secret, hex. */
    data class Pc(val id: String, val name: String, val key: String)

    fun phoneId(ctx: Context): String {
        val p = sp(ctx)
        return p.getString("phone_id", null) ?: UUID.randomUUID().toString().replace("-", "").take(24).also {
            p.edit().putString("phone_id", it).apply()
        }
    }

    /** The name computers see: the device name from Settings, else the model. */
    fun phoneName(ctx: Context): String =
        runCatching { Settings.Global.getString(ctx.contentResolver, Settings.Global.DEVICE_NAME) }.getOrNull()
            ?.takeIf { it.isNotBlank() } ?: Build.MODEL

    fun pcs(ctx: Context): List<Pc> {
        val arr = runCatching { JSONArray(sp(ctx).getString("pcs", "[]")) }.getOrNull() ?: return emptyList()
        return (0 until arr.length()).map {
            val o = arr.getJSONObject(it)
            Pc(o.optString("id"), o.optString("name"), o.optString("key"))
        }.filter { it.id.isNotEmpty() && it.key.length == 64 }
    }

    fun savePc(ctx: Context, pc: Pc) = writePcs(ctx, pcs(ctx).filter { it.id != pc.id } + pc)

    fun removePc(ctx: Context, id: String) = writePcs(ctx, pcs(ctx).filter { it.id != id })

    private fun writePcs(ctx: Context, list: List<Pc>) {
        sp(ctx).edit().putString("pcs", pcsJson(list)).apply()
    }

    /** The JSON the Go side wants for BoostSetPeers. */
    fun pcsJson(list: List<Pc>): String {
        val arr = JSONArray()
        for (pc in list) arr.put(JSONObject().put("id", pc.id).put("name", pc.name).put("key", pc.key))
        return arr.toString()
    }

    /** "system", "light" or "dark". */
    fun theme(ctx: Context): String = sp(ctx).getString("theme", "system") ?: "system"
    fun setTheme(ctx: Context, v: String) = sp(ctx).edit().putString("theme", v).apply()

    fun evenSplit(ctx: Context): Boolean = sp(ctx).getBoolean("even_split", false)
    fun setEvenSplit(ctx: Context, on: Boolean) = sp(ctx).edit().putBoolean("even_split", on).apply()
}
