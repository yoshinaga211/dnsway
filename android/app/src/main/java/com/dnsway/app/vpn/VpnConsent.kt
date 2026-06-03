package com.dnsway.app.vpn

import android.content.Context

object VpnConsent {
    private const val PREF_NAME = "dnsway_vpn_consent"
    private const val KEY_CONSENTED = "user_consented"

    fun isConsented(context: Context): Boolean {
        return context.getSharedPreferences(PREF_NAME, Context.MODE_PRIVATE)
            .getBoolean(KEY_CONSENTED, false)
    }

    fun setConsented(context: Context) {
        context.getSharedPreferences(PREF_NAME, Context.MODE_PRIVATE)
            .edit()
            .putBoolean(KEY_CONSENTED, true)
            .apply()
    }
}
