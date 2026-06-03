package com.dnsway.app.ui.screens

import android.app.Activity
import android.app.admin.DevicePolicyManager
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.os.Build
import android.provider.Settings
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.dnsway.app.admin.DnswayDeviceAdminReceiver
import com.dnsway.app.ui.theme.*
import com.dnsway.app.vpn.LocalDnsVpnService
import com.dnsway.app.vpn.VpnConsent
import com.dnsway.app.vpn.VpnGuardService

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SecuritySetupScreen(onBack: () -> Unit) {
    val context = LocalContext.current
    val isVpnActive = LocalDnsVpnService.isRunning
    var isAdminActive by remember { mutableStateOf(DnswayDeviceAdminReceiver.isActive(context)) }
    var isGuardActive by remember { mutableStateOf(isAccessibilityServiceEnabled(context)) }

    // DeviceAdmin launcher
    val adminLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.StartActivityForResult()
    ) {
        isAdminActive = DnswayDeviceAdminReceiver.isActive(context)
    }

    // VPN consent + launcher
    val vpnLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK) {
            LocalDnsVpnService.start(context)
        }
    }
    var showVpnConsent by remember { mutableStateOf(false) }

    fun requestVpnStart() {
        if (VpnConsent.isConsented(context)) {
            val intent = LocalDnsVpnService.prepare(context)
            if (intent != null) {
                vpnLauncher.launch(intent)
            } else {
                LocalDnsVpnService.start(context)
            }
        } else {
            showVpnConsent = true
        }
    }

    if (showVpnConsent) {
        AlertDialog(
            onDismissRequest = { showVpnConsent = false },
            title = { Text("使用 VPN 模式") },
            text = {
                Text(
                    "Dnsway 使用 Android 本地 VPN 模式实现 DNS 过滤。\n\n" +
                    "• 所有 DNS 查询在设备本地完成过滤\n" +
                    "• 不会将流量路由到外部服务器\n" +
                    "• 不收集任何个人身份信息\n" +
                    "• 仅用于家长控制目的\n\n" +
                    "是否同意并继续？"
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    VpnConsent.setConsented(context)
                    showVpnConsent = false
                    val intent = LocalDnsVpnService.prepare(context)
                    if (intent != null) {
                        vpnLauncher.launch(intent)
                    } else {
                        LocalDnsVpnService.start(context)
                    }
                }) {
                    Text("同意并继续")
                }
            },
            dismissButton = {
                TextButton(onClick = { showVpnConsent = false }) {
                    Text("拒绝")
                }
            }
        )
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("安全加固") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                }
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(padding)
                .padding(20.dp)
        ) {
            // Title
            Text(
                text = "构建坚固堡垒",
                fontSize = 22.sp,
                fontWeight = FontWeight.Bold,
                color = Gray900
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = "三层防护，防止过滤被绕过",
                fontSize = 14.sp,
                color = Gray500
            )

            Spacer(modifier = Modifier.height(24.dp))

            // Layer 1: VPN
            SecurityLayerCard(
                layer = "1",
                title = "VPN 过滤",
                description = "拦截所有 DNS 查询，过滤成人内容和游戏",
                isActive = isVpnActive,
                primaryAction = "启动 VPN 过滤",
                onPrimaryAction = { requestVpnStart() },
                secondaryAction = if (isVpnActive) "停止过滤" else null,
                onSecondaryAction = if (isVpnActive) {{ LocalDnsVpnService.stop(context) }} else null
            )

            Spacer(modifier = Modifier.height(16.dp))

            // Layer 2: DeviceAdmin
            SecurityLayerCard(
                layer = "2",
                title = "设备管理员",
                description = "激活后无法直接卸载应用，需先关闭管理员权限",
                isActive = isAdminActive,
                primaryAction = "激活设备管理员",
                onPrimaryAction = {
                    val intent = DnswayDeviceAdminReceiver.getActivationIntent(context)
                    adminLauncher.launch(intent)
                },
                secondaryAction = null,
                onSecondaryAction = null
            )

            Spacer(modifier = Modifier.height(16.dp))

            // Layer 3: AccessibilityService
            SecurityLayerCard(
                layer = "3",
                title = "辅助功能守护",
                description = "自动检测 VPN 状态，监控设置页面绕过行为",
                isActive = isGuardActive,
                primaryAction = if (!isGuardActive) "开启辅助功能" else null,
                onPrimaryAction = {
                    val intent = Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)
                    context.startActivity(intent)
                },
                secondaryAction = null,
                onSecondaryAction = null
            )

            Spacer(modifier = Modifier.height(24.dp))

            InfoCard(context)
        }
    }
}

@Composable
private fun SecurityLayerCard(
    layer: String,
    title: String,
    description: String,
    isActive: Boolean,
    primaryAction: String?,
    onPrimaryAction: (() -> Unit)?,
    secondaryAction: String?,
    onSecondaryAction: (() -> Unit)?
) {
    val bgColor = if (isActive) GreenBg else Color.White

    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = bgColor),
        elevation = CardDefaults.cardElevation(defaultElevation = if (isActive) 0.dp else 1.dp)
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                // Layer badge
                Surface(
                    shape = androidx.compose.foundation.shape.CircleShape,
                    color = if (isActive) Green500 else Gray300,
                    modifier = Modifier.size(32.dp)
                ) {
                    Box(contentAlignment = Alignment.Center) {
                        Text(
                            text = if (isActive) "✓" else layer,
                            color = Color.White,
                            fontSize = 14.sp,
                            fontWeight = FontWeight.Bold
                        )
                    }
                }
                Spacer(modifier = Modifier.width(12.dp))
                Column(modifier = Modifier.weight(1f)) {
                    Text(text = title, fontSize = 16.sp, fontWeight = FontWeight.SemiBold, color = Gray900)
                    Text(
                        text = if (isActive) "已激活" else "未激活",
                        fontSize = 12.sp,
                        color = if (isActive) Green500 else Gray500
                    )
                }
            }

            Spacer(modifier = Modifier.height(8.dp))

            Text(
                text = description,
                fontSize = 13.sp,
                color = Gray700,
                lineHeight = 20.sp
            )

            if (primaryAction != null || secondaryAction != null) {
                Spacer(modifier = Modifier.height(12.dp))
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    if (primaryAction != null && onPrimaryAction != null) {
                        Button(
                            onClick = onPrimaryAction,
                            modifier = Modifier.weight(1f),
                            colors = ButtonDefaults.buttonColors(
                                containerColor = if (isActive) Green500 else Blue500
                            )
                        ) {
                            Text(primaryAction, fontSize = 13.sp)
                        }
                    }
                    if (secondaryAction != null && onSecondaryAction != null) {
                        OutlinedButton(
                            onClick = onSecondaryAction,
                            modifier = Modifier.weight(1f)
                        ) {
                            Text(secondaryAction, fontSize = 13.sp, color = Red500)
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun InfoCard(context: Context) {
    val isGuardActive = isAccessibilityServiceEnabled(context)

    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = OrangeBg)
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(
                text = "💡 提示",
                fontSize = 14.sp,
                fontWeight = FontWeight.SemiBold,
                color = Orange500
            )
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = buildString {
                    append("1. 先启用 VPN 过滤\n")
                    append("2. 再激活设备管理员（防止卸载）\n")
                    append("3. 最后开启辅助功能（自动守护）\n\n")
                    append("三层全部激活后，过滤无法被绕过。\n")
                    if (!isGuardActive) {
                        append("\n开启辅助功能步骤：\n")
                        append("设置 → 辅助功能 → 已安装的应用 → Dnsway → 开启")
                    }
                },
                fontSize = 13.sp,
                color = Gray700,
                lineHeight = 20.sp
            )
        }
    }
}

private fun isAccessibilityServiceEnabled(context: Context): Boolean {
    val service = "${context.packageName}/.vpn.VpnGuardService"
    return try {
        val enabledServices = Settings.Secure.getString(
            context.contentResolver,
            Settings.Secure.ENABLED_ACCESSIBILITY_SERVICES
        ) ?: ""
        enabledServices.split(':').any { it.equals(service, ignoreCase = true) }
    } catch (e: Exception) {
        false
    }
}
