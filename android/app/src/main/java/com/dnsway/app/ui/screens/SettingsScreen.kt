package com.dnsway.app.ui.screens

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.KeyboardArrowRight
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.dnsway.app.DnswayApp
import com.dnsway.app.engine.DnsEngine
import com.dnsway.app.ui.theme.*
import com.dnsway.app.vpn.LocalDnsVpnService
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(
    onNavigateToCategories: () -> Unit = {},
    onNavigateToLogs: () -> Unit = {},
    onNavigateToSecurity: () -> Unit = {},
    onNavigateToPrivacy: () -> Unit = {},
    onRequirePin: ((action: () -> Unit) -> Unit)? = null
) {
    val scope = rememberCoroutineScope()
    val dao = DnswayApp.instance.database.ruleDao()
    val engineStats = remember { DnsEngine.getDailyStats() }
    val safeSearch by DnsEngine.safeSearchEnabled.collectAsState()

    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(16.dp)
    ) {
        // Filter settings section
        Text(
            text = "过滤设置",
            fontSize = 16.sp,
            fontWeight = FontWeight.SemiBold,
            color = Gray900
        )
        Spacer(modifier = Modifier.height(10.dp))

        // Safe search toggle
        Card(
            modifier = Modifier.fillMaxWidth(),
            colors = CardDefaults.cardColors(containerColor = Color.White),
            elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 10.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text("安全搜索", fontSize = 14.sp, color = Gray900, fontWeight = FontWeight.Medium)
                    Text("强制 Google/Bing 安全搜索", fontSize = 12.sp, color = Gray500)
                }
                Switch(
                    checked = safeSearch,
                    onCheckedChange = { DnsEngine.setSafeSearchEnabled(it) },
                    colors = SwitchDefaults.colors(
                        checkedTrackColor = Green500,
                        checkedThumbColor = Color.White
                    )
                )
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        // Category management
        ClickableSettingsItem(
            text = "分类管理",
            subtitle = "开关各分类的域名拦截",
            onClick = onNavigateToCategories
        )

        Spacer(modifier = Modifier.height(8.dp))

        // Query logs
        ClickableSettingsItem(
            text = "查询日志",
            subtitle = "查看 DNS 查询历史记录",
            onClick = onNavigateToLogs
        )

        Spacer(modifier = Modifier.height(8.dp))

        // Security setup
        ClickableSettingsItem(
            text = "安全加固",
            subtitle = "设备管理员 + 辅助功能守护，防止绕过",
            onClick = onNavigateToSecurity
        )

        Spacer(modifier = Modifier.height(24.dp))

        // App Info section
        Text(
            text = "应用信息",
            fontSize = 16.sp,
            fontWeight = FontWeight.SemiBold,
            color = Gray900
        )
        Spacer(modifier = Modifier.height(10.dp))

        SettingsItem("版本", "1.0.0")
        SettingsItem("引擎状态", if (DnsEngine.isInitialized) "运行中" else "未启动")
        SettingsItem("VPN 状态", if (LocalDnsVpnService.isRunning) "已启用" else "已停用")
        SettingsItem("本地统计", "查询 ${engineStats.totalQueries} / 拦截 ${engineStats.blockedQueries}")

        Spacer(modifier = Modifier.height(24.dp))

        // About section
        Text(
            text = "关于",
            fontSize = 16.sp,
            fontWeight = FontWeight.SemiBold,
            color = Gray900
        )
        Spacer(modifier = Modifier.height(10.dp))

        Card(
            modifier = Modifier.fillMaxWidth(),
            colors = CardDefaults.cardColors(containerColor = Color.White),
            elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
        ) {
            Column(modifier = Modifier.padding(16.dp)) {
                Text(
                    text = "Dnsway - 本地 DNS 过滤",
                    fontSize = 14.sp,
                    fontWeight = FontWeight.Medium,
                    color = Gray900
                )
                Spacer(modifier = Modifier.height(4.dp))
                Text(
                    text = "使用本地 VPN 模式拦截 DNS 查询，\n" +
                            "内置过滤引擎在设备本地完成判断。\n" +
                            "无需外部服务器，不依赖网络连接。",
                    fontSize = 13.sp,
                    color = Gray700,
                    lineHeight = 20.sp
                )
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        // Privacy policy
        ClickableSettingsItem(
            text = "隐私政策",
            subtitle = "查看隐私政策与数据处理说明",
            onClick = onNavigateToPrivacy
        )

        Spacer(modifier = Modifier.height(24.dp))

        // Data management
        Text(
            text = "数据管理",
            fontSize = 16.sp,
            fontWeight = FontWeight.SemiBold,
            color = Red500
        )
        Spacer(modifier = Modifier.height(10.dp))

        OutlinedButton(
            onClick = {
                if (onRequirePin != null) {
                    onRequirePin { scope.launch { dao.clearLogs() } }
                } else {
                    scope.launch { dao.clearLogs() }
                }
            },
            modifier = Modifier.fillMaxWidth(),
            colors = ButtonDefaults.outlinedButtonColors(contentColor = Red500)
        ) {
            Text("清除本地查询日志")
        }
    }
}

@Composable
private fun ClickableSettingsItem(
    text: String,
    subtitle: String,
    onClick: () -> Unit
) {
    Card(
        onClick = onClick,
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = Color.White),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 14.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(text = text, fontSize = 14.sp, fontWeight = FontWeight.Medium, color = Gray900)
                Text(text = subtitle, fontSize = 12.sp, color = Gray500)
            }
            Icon(
                Icons.Default.KeyboardArrowRight,
                contentDescription = null,
                tint = Gray300,
                modifier = Modifier.size(20.dp)
            )
        }
    }
}

@Composable
private fun SettingsItem(label: String, value: String) {
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 3.dp),
        colors = CardDefaults.cardColors(containerColor = Color.White),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 14.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text(text = label, fontSize = 14.sp, color = Gray700)
            Text(text = value, fontSize = 14.sp, fontWeight = FontWeight.Medium, color = Gray900)
        }
    }
}
