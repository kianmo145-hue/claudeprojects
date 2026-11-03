package org.paydar.app

import android.app.Application

class PaydarApp : Application() {
    override fun onCreate() {
        super.onCreate()
        Core.init(this)
    }
}
