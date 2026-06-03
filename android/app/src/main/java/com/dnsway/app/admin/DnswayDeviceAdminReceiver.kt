package com.dnsway.app.admin

import android.app.admin.DeviceAdminReceiver
import android.app.admin.DevicePolicyManager
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.util.Log
import android.widget.Toast

class DnswayDeviceAdminReceiver : DeviceAdminReceiver() {

    override fun onEnabled(context: Context, intent: Intent) {
        Log.i(TAG, "Device admin enabled")
        Toast.makeText(context, "设备管理员已激活", Toast.LENGTH_SHORT).show()
    }

    override fun onDisabled(context: Context, intent: Intent) {
        Log.w(TAG, "Device admin disabled — app can now be uninstalled")
        Toast.makeText(context, "设备管理员已关闭", Toast.LENGTH_SHORT).show()
    }

    override fun onDisableRequested(context: Context, intent: Intent): CharSequence {
        return "关闭设备管理员后，应用可以被卸载。确定要关闭吗？"
    }

    companion object {
        const val TAG = "DnswayAdmin"

        fun getActivationIntent(context: Context): Intent {
            return Intent(DevicePolicyManager.ACTION_ADD_DEVICE_ADMIN).apply {
                putExtra(DevicePolicyManager.EXTRA_DEVICE_ADMIN,
                    ComponentName(context, DnswayDeviceAdminReceiver::class.java))
                putExtra(DevicePolicyManager.EXTRA_ADD_EXPLANATION,
                    "激活后防止应用被随意卸载，确保过滤持续生效")
            }
        }

        fun isActive(context: Context): Boolean {
            val mgr = context.getSystemService(DevicePolicyManager::class.java)
            val cn = ComponentName(context, DnswayDeviceAdminReceiver::class.java)
            return mgr.isAdminActive(cn)
        }
    }
}
