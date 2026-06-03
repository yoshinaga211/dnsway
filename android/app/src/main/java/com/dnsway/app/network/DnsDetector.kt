package com.dnsway.app.network

import android.content.Context

data class DnsStatus(
    val vpnActive: Boolean = false,
    val error: String = ""
)

enum class ProtectionLevel {
    PROTECTED,
    NOT_CONFIGURED,
    BYPASSED,
    ERROR
}

object DnsDetector {

    /**
     * Check the device's DNS filtering status.
     * No server calls — purely checks if the local VPN is active.
     */
    suspend fun checkStatus(context: Context): DnsStatus {
        return DnsStatus(
            vpnActive = com.dnsway.app.vpn.LocalDnsVpnService.isRunning
        )
    }

    fun getProtectionLevel(status: DnsStatus): ProtectionLevel {
        return when {
            status.vpnActive -> ProtectionLevel.PROTECTED
            else -> ProtectionLevel.NOT_CONFIGURED
        }
    }
}
