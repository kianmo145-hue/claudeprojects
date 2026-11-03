package org.paydar.app

import android.Manifest
import android.annotation.SuppressLint
import android.app.Activity
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Color
import android.net.Uri
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import android.view.View
import android.webkit.JavascriptInterface
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.TextView
import android.widget.Toast

/** Shows the shared UI (served by the Go core on 127.0.0.1) in a WebView. */
class MainActivity : Activity() {

    companion object {
        private const val REQ_VPN = 1
        private const val REQ_NOTIF = 2
    }

    private var web: WebView? = null

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        Core.attach(this)
        val url = Core.uiUrl
        if (url == null) {
            setContentView(TextView(this).apply {
                text = "Paydar could not start: ${Core.startError}"
                setTextColor(Color.WHITE)
                setPadding(48, 96, 48, 48)
            })
            return
        }
        val w = WebView(this)
        w.setBackgroundColor(Color.parseColor("#0B1117"))
        w.settings.javaScriptEnabled = true
        w.settings.domStorageEnabled = true
        w.settings.allowFileAccess = false
        w.settings.allowContentAccess = false
        w.overScrollMode = View.OVER_SCROLL_NEVER
        w.addJavascriptInterface(Bridge(), "PaydarNative")
        w.webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                val u = request.url
                if (u.host == "127.0.0.1") return false
                // Anything else (help links) opens in the browser, never inside the app.
                runCatching { startActivity(Intent(Intent.ACTION_VIEW, u)) }
                return true
            }
        }
        setContentView(w)
        w.loadUrl(url)
        web = w
        handleShared(intent)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleShared(intent)
    }

    /** A config link shared to the app or opened from a browser goes to the clipboard flow. */
    private fun handleShared(intent: Intent?) {
        val text = intent?.getStringExtra(Intent.EXTRA_TEXT) ?: intent?.data?.toString() ?: return
        val cm = getSystemService(ClipboardManager::class.java)
        cm.setPrimaryClip(ClipData.newPlainText("config", text))
    }

    override fun onResume() {
        super.onResume()
        Core.attach(this)
    }

    override fun onDestroy() {
        Core.attach(null)
        web?.destroy()
        super.onDestroy()
    }

    @Deprecated("Deprecated in Java")
    override fun onBackPressed() {
        // Keep the app (and its WebView) alive; the VPN runs in its service anyway.
        moveTaskToBack(true)
    }

    fun connectWithPermission() {
        if (Build.VERSION.SDK_INT >= 33 &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), REQ_NOTIF)
            // The VPN does not need the notification permission; continue either way.
        }
        val prep = VpnService.prepare(this)
        if (prep != null) {
            startActivityForResult(prep, REQ_VPN)
        } else {
            PaydarVpnService.start(this)
        }
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == REQ_VPN) {
            if (resultCode == RESULT_OK) {
                PaydarVpnService.start(this)
            } else {
                Toast.makeText(this, R.string.vpn_denied, Toast.LENGTH_LONG).show()
            }
        }
    }

    /** What the page can ask of Android (window.PaydarNative). */
    inner class Bridge {
        @JavascriptInterface
        fun readClipboard(): String {
            val cm = getSystemService(ClipboardManager::class.java)
            val clip = cm.primaryClip ?: return ""
            if (clip.itemCount == 0) return ""
            return clip.getItemAt(0).coerceToText(this@MainActivity)?.toString() ?: ""
        }

        @JavascriptInterface
        fun copy(text: String) {
            val cm = getSystemService(ClipboardManager::class.java)
            cm.setPrimaryClip(ClipData.newPlainText("Paydar", text))
        }

        @JavascriptInterface
        fun openUrl(url: String) {
            runOnUiThread { runCatching { startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url))) } }
        }
    }
}
