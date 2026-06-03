package com.dnsway.app.ui.screens

import android.app.Activity
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.dnsway.app.DnswayApp
import com.dnsway.app.engine.DnsEngine
import com.dnsway.app.network.ProtectionLevel
import com.dnsway.app.ui.components.*
import com.dnsway.app.ui.theme.*
import com.dnsway.app.vpn.LocalDnsVpnService
import com.dnsway.app.vpn.VpnConsent
import java.text.SimpleDateFormat
import java.util.*

@Composable
fun HomeScreen(
    onNavigateToGuide: () -> Unit,
    onNavigateToLogs: () -> Unit = {},
    onRequirePin: ((action: () -> Unit) -> Unit)? = null
) {
    val dao = DnswayApp.instance.database.ruleDao()
    val recentLogs by dao.getRecentLogs().collectAsState(initial = emptyList())
    val context = LocalContext.current
    val isVpnRunning = LocalDnsVpnService.isRunning

    // Stats from local engine
    val stats = DnsEngine.getDailyStats()

    // VPN consent request
    val vpnLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK) {
            LocalDnsVpnService.start(context)
        }
    }

    // User consent dialog
    var showConsentDialog by remember { mutableStateOf(false) }

    fun requestVpnStart() {
        if (VpnConsent.isConsented(context)) {
            val intent = LocalDnsVpnService.prepare(context)
            if (intent != null) {
                vpnLauncher.launch(intent)
            } else {
                LocalDnsVpnService.start(context)
            }
        } else {
            showConsentDialog = true
        }
    }

    // Auto-start VPN on first composition
    LaunchedEffect(Unit) {
        if (!isVpnRunning) {
            requestVpnStart()
        }
    }

    if (showConsentDialog) {
        AlertDialog(
            onDismissRequest = { showConsentDialog = false },
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
                    showConsentDialog = false
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
                TextButton(onClick = { showConsentDialog = false }) {
                    Text("拒绝")
                }
            }
        )
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(16.dp)
    ) {
        // Header
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column {
                Text("Dnsway", fontSize = 24.sp, fontWeight = FontWeight.Bold, color = Gray900)
                Text("本地 DNS 过滤", fontSize = 14.sp, color = Gray500)
            }

            // VPN toggle switch
            Switch(
                checked = isVpnRunning,
                onCheckedChange = { enable ->
                    if (enable) {
                        requestVpnStart()
                    } else {
                        LocalDnsVpnService.stop(context)
                    }
                },
                colors = SwitchDefaults.colors(checkedTrackColor = Green500)
            )
        }

        Spacer(modifier = Modifier.height(20.dp))

        // Status card
        StatusCard(
            level = if (isVpnRunning) ProtectionLevel.PROTECTED else ProtectionLevel.NOT_CONFIGURED,
            latencyMs = 0,
            onClick = { if (!isVpnRunning) onNavigateToGuide() }
        )

        Spacer(modifier = Modifier.height(16.dp))

        // Stats from local engine
        StatsCard(
            totalQueries = stats.totalQueries,
            blockedCount = stats.blockedQueries
        )

        Spacer(modifier = Modifier.height(20.dp))

        // Quick Actions
        Text("快速操作", fontSize = 16.sp, fontWeight = FontWeight.SemiBold, color = Gray900)
        Spacer(modifier = Modifier.height(10.dp))

        if (isVpnRunning) {
            QuickActionButton(text = "停止过滤") {
                if (onRequirePin != null) {
                    onRequirePin { LocalDnsVpnService.stop(context) }
                } else {
                    LocalDnsVpnService.stop(context)
                }
            }
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "DNS 过滤已启用 ✓",
                fontSize = 12.sp,
                color = Green500
            )
        } else {
            QuickActionButton(text = "启用本地 DNS 过滤") {
                requestVpnStart()
            }
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "启动 VPN 模式，在本地过滤 DNS 查询",
                fontSize = 12.sp,
                color = Gray500
            )
        }

        Spacer(modifier = Modifier.height(24.dp))

        // Recent queries
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text("最近查询", fontSize = 16.sp, fontWeight = FontWeight.SemiBold, color = Gray900)
            TextButton(onClick = onNavigateToLogs) {
                Text("查看全部", fontSize = 13.sp, color = Blue500)
            }
        }
        Spacer(modifier = Modifier.height(8.dp))

        if (recentLogs.isEmpty()) {
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = Gray100)
            ) {
                Box(
                    modifier = Modifier.fillMaxWidth().padding(24.dp),
                    contentAlignment = Alignment.Center
                ) {
                    Text("暂无查询记录", fontSize = 13.sp, color = Gray500)
                }
            }
        } else {
            recentLogs.take(5).forEach { log ->
                RecentQueryItem(log = log)
                Spacer(modifier = Modifier.height(4.dp))
            }
        }

        Spacer(modifier = Modifier.height(24.dp))

        // Info card
        Card(
            modifier = Modifier.fillMaxWidth(),
            colors = CardDefaults.cardColors(containerColor = BlueBg)
        ) {
            Column(modifier = Modifier.padding(16.dp)) {
                Text(
                    text = "工作原理",
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = Gray900
                )
                Spacer(modifier = Modifier.height(6.dp))
                Text(
                    text = "Dnsway 在本地创建 VPN，拦截全部 DNS 查询。\n" +
                            "内置过滤引擎自动判断每个域名是否放行。\n" +
                            "无需外部服务器，所有处理在设备本地完成。",
                    fontSize = 13.sp,
                    color = Gray700,
                    lineHeight = 22.sp
                )
            }
        }
    }
}

@Composable
private fun RecentQueryItem(log: com.dnsway.app.data.models.QueryLog) {
    val isBlocked = log.decision == "BLOCK"
    val timeStr = remember(log.timestamp) {
        val sdf = SimpleDateFormat("HH:mm:ss", Locale.getDefault())
        sdf.format(Date(log.timestamp))
    }

    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = androidx.compose.ui.graphics.Color.White),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Surface(
                shape = androidx.compose.foundation.shape.RoundedCornerShape(4.dp),
                color = if (isBlocked) RedBg else GreenBg,
                modifier = Modifier.size(24.dp)
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Text(
                        text = if (isBlocked) "拦" else "放",
                        fontSize = 10.sp,
                        fontWeight = FontWeight.Bold,
                        color = if (isBlocked) Red500 else Green500
                    )
                }
            }
            Spacer(modifier = Modifier.width(8.dp))
            Text(
                text = log.domain,
                fontSize = 13.sp,
                color = Gray900,
                modifier = Modifier.weight(1f),
                maxLines = 1,
                overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis
            )
            Text(
                text = timeStr,
                fontSize = 11.sp,
                color = Gray300
            )
        }
    }
}
