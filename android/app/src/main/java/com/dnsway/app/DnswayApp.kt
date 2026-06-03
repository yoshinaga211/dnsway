package com.dnsway.app

import android.app.Application
import com.dnsway.app.data.AppDatabase
import com.dnsway.app.engine.DnsEngine

class DnswayApp : Application() {

    val database: AppDatabase by lazy {
        AppDatabase.getInstance(this)
    }

    override fun onCreate() {
        super.onCreate()
        instance = this
        // Initialize the Go DNS filtering engine (gracefully handles missing native lib)
        DnsEngine.initialize(this)
    }

    companion object {
        lateinit var instance: DnswayApp
            private set
    }
}
