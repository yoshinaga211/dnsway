package com.dnsway.app.vpn

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.AccessibilityServiceInfo
import android.content.Intent
import android.provider.Settings
import android.util.Log
import android.view.accessibility.AccessibilityEvent
import com.dnsway.app.DnswayApp
import kotlinx.coroutines.*

/**
 * AccessibilityService that monitors VPN filtering status and detects
 * bypass attempts by the child.
 *
 * Detection scope:
 * - System Settings pages that could lead to bypass (VPN, Accessibility, App info, Private DNS)
 * - VPN connectivity watchdog (checks every 5 s)
 * - Unexpected restart / force-stop detection
 * - USB debugging status
 */
class VpnGuardService : AccessibilityService() {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var watchdogJob: Job? = null
    private var restartAttempts = 0

    override fun onServiceConnected() {
        Log.i(TAG, "Guard service connected")
        isRunning = true

        val info = AccessibilityServiceInfo().apply {
            eventTypes = AccessibilityEvent.TYPES_ALL_MASK
            feedbackType = AccessibilityServiceInfo.FEEDBACK_GENERIC
            notificationTimeout = 1000
        }
        serviceInfo = info

        // Check if VPN was running before a potential force-stop
        checkUnexpectedRestart()

        // Warn about USB debugging
        checkUsbDebugging()

        // Periodic watchdog
        watchdogJob = scope.launch {
            while (isActive) {
                delay(5000)
                checkVpnStatus()
            }
        }
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {
        if (event == null) return

        when (event.eventType) {
            AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED -> {
                val packageName = event.packageName?.toString() ?: return
                val className = event.className?.toString() ?: ""
                handleWindowChange(packageName, className)
            }
        }
    }

    override fun onInterrupt() {
        Log.w(TAG, "Guard service interrupted")
    }

    override fun onDestroy() {
        Log.i(TAG, "Guard service destroyed")
        isRunning = false
        watchdogJob?.cancel()
        scope.cancel()
        super.onDestroy()
    }

    // ── Window monitoring ─────────────────────────────────────────────

    private fun handleWindowChange(packageName: String, className: String) {
        if (packageName != "com.android.settings") return

        when {
            className.contains("VpnSettings") || className.contains("VpnFragment") ->
                notify("VPN 设置页面被打开")

            className.contains("AccessibilitySettings") || className.contains("AccessibilityFragment") ->
                notify("辅助功能设置页面被打开")

            className.contains("InstalledAppDetails") || className.contains("ApplicationInfo") ->
                notify("应用详情页面被打开")

            className.contains("PrivateDns") || className.contains("PrivateDnsSettings") ->
                notify("私有 DNS（DoT）设置页面被打开")

            className.contains("SecuritySettings") || className.contains("SecurityHub") ->
                notify("安全设置页面被打开")
        }
    }

    // ── Watchdog ──────────────────────────────────────────────────────

    private fun checkVpnStatus() {
        if (!LocalDnsVpnService.isRunning) {
            Log.w(TAG, "VPN is not running! Attempting to restore...")
            notify("VPN 过滤已停止")
            // Try to restart — may succeed on some Android versions
            tryAutoRestart()
        }
    }

    private fun tryAutoRestart() {
        if (restartAttempts >= 3) return
        restartAttempts++

        val intent = LocalDnsVpnService.prepare(this)
        if (intent == null) {
            // Already prepared — try direct start
            try {
                LocalDnsVpnService.start(this)
                Log.i(TAG, "VPN auto-restart attempt $restartAttempts")
            } catch (e: Exception) {
                Log.w(TAG, "Auto-restart failed: ${e.message}")
            }
        }
        // If intent != null, we need user consent — can't auto-start
    }

    // ── Startup checks ────────────────────────────────────────────────

    /** Detect if the app was force-stopped by checking persisted VPN state. */
    private fun checkUnexpectedRestart() {
        val prefs = DnswayApp.instance.getSharedPreferences(PREF_NAME, 0)
        val wasRunning = prefs.getBoolean(PREF_VPN_WAS_RUNNING, false)
        if (wasRunning && !LocalDnsVpnService.isRunning) {
            Log.w(TAG, "VPN was running before restart but is now down — possible force-stop")
            notify("检测到应用可能被强制关闭")
        }
        // Mark that we're running now
        prefs.edit().putBoolean(PREF_VPN_WAS_RUNNING, true).apply()
    }

    private fun checkUsbDebugging() {
        try {
            val adbEnabled = Settings.Secure.getInt(
                contentResolver,
                Settings.Secure.ADB_ENABLED
            ) == 1
            if (adbEnabled) {
                Log.w(TAG, "USB debugging is enabled — app can be uninstalled via ADB")
                notify("USB 调试已开启，应用可通过电脑卸载")
            }
        } catch (_: Exception) {}
    }

    // ── Notifications ─────────────────────────────────────────────────

    private fun notify(text: String) {
        val notification = android.app.Notification.Builder(this, NOTIFICATION_CHANNEL_ID)
            .setContentTitle("Dnsway 安全提醒")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_dialog_alert)
            .setAutoCancel(true)
            .build()
        getSystemService(android.app.NotificationManager::class.java)
            .notify(System.currentTimeMillis().toInt(), notification)
    }

    companion object {
        const val TAG = "DnswayGuard"
        const val NOTIFICATION_CHANNEL_ID = "dnsway_guard"
        private const val PREF_NAME = "dnsway_guard"
        private const val PREF_VPN_WAS_RUNNING = "vpn_was_running"

        @Volatile var isRunning = false
            private set
    }
}
