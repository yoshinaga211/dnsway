package com.dnsway.app.ui.screens

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.net.VpnService
import android.os.Build
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
import com.dnsway.app.network.ProtectionLevel
import com.dnsway.app.ui.theme.*
import com.dnsway.app.vpn.LocalDnsVpnService

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DnsGuideScreen(
    onBack: () -> Unit,
    onNavigateToSecurity: () -> Unit = {}
) {
    val context = LocalContext.current
    val isVpnActive = LocalDnsVpnService.isRunning

    val vpnLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK) {
            LocalDnsVpnService.start(context)
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("启用本地过滤") },
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
            // Current status
            StatusBanner(vpnActive = isVpnActive)

            Spacer(modifier = Modifier.height(24.dp))

            // How it works
            Text(
                text = "工作原理",
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                color = Gray900
            )
            Spacer(modifier = Modifier.height(12.dp))

            GuideStep(
                step = "1",
                title = "启用 VPN 过滤",
                description = "点击下方按钮，系统会弹出 VPN 连接请求",
                details = "Dnsway 在本地创建 VPN，不会将你的数据发送到外部服务器。"
            )

            Spacer(modifier = Modifier.height(12.dp))

            GuideStep(
                step = "2",
                title = "自动拦截",
                description = "所有 DNS 查询经过内置过滤引擎",
                details = "引擎按优先级判断：白名单 → 黑名单 → 分类屏蔽（成人/游戏/社交等8类）→ 默认放行"
            )

            Spacer(modifier = Modifier.height(12.dp))

            GuideStep(
                step = "3",
                title = "查询放行",
                description = "允许的查询转发到 1.1.1.1 获取真实结果",
                details = "被拦截的域名返回 NXDOMAIN（域名不存在）"
            )

            Spacer(modifier = Modifier.height(24.dp))

            // Action button
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = if (isVpnActive) GreenBg else BlueBg)
            ) {
                Column(
                    modifier = Modifier.padding(16.dp),
                    horizontalAlignment = Alignment.CenterHorizontally
                ) {
                    Text(
                        text = if (isVpnActive) "过滤已启用" else "启动过滤",
                        fontSize = 16.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = if (isVpnActive) Green500 else Gray900
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(
                        text = if (isVpnActive)
                            "Dnsway 正在本地过滤 DNS 查询"
                        else
                            "点击下方按钮启用本地 DNS 过滤，无需外部服务器",
                        fontSize = 13.sp,
                        color = Gray700,
                        textAlign = TextAlign.Center
                    )
                    Spacer(modifier = Modifier.height(12.dp))

                    if (isVpnActive) {
                        OutlinedButton(onClick = { LocalDnsVpnService.stop(context) }) {
                            Text("停止过滤", color = Red500)
                        }
                    } else {
                        Button(onClick = {
                            val intent = LocalDnsVpnService.prepare(context)
                            if (intent != null) {
                                vpnLauncher.launch(intent)
                            } else {
                                LocalDnsVpnService.start(context)
                            }
                        }) {
                            Text("启用本地 DNS 过滤")
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.height(24.dp))

            // Tips
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = OrangeBg)
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = "提示",
                        fontSize = 14.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = Orange500
                    )
                    Spacer(modifier = Modifier.height(6.dp))
                    Text(
                        text = "VPN 模式仅在应用开启时工作。\n\n" +
                                "首次启用需要同意系统 VPN 权限请求。\n\n" +
                                "过滤完全在本地完成，不需要网络连接。",
                        fontSize = 13.sp,
                        color = Gray700,
                        lineHeight = 20.sp
                    )
                }
            }

            Spacer(modifier = Modifier.height(16.dp))

            // Fortress setup link
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = BlueBg)
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = "🔒 安全加固",
                        fontSize = 14.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = Gray900
                    )
                    Spacer(modifier = Modifier.height(6.dp))
                    Text(
                        text = "激活设备管理员（防卸载）和辅助功能（自动守护 VPN），构建三层防护堡垒。",
                        fontSize = 13.sp,
                        color = Gray700,
                        lineHeight = 20.sp
                    )
                    Spacer(modifier = Modifier.height(10.dp))
                    OutlinedButton(onClick = onNavigateToSecurity) {
                        Text("前往加固", fontSize = 13.sp)
                    }
                }
            }
        }
    }
}

@Composable
private fun StatusBanner(vpnActive: Boolean) {
    val (icon, label, color, bgColor) = if (vpnActive) {
        arrayOf("✅", "已保护 - Dnsway 正在本地过滤 DNS", Green500, GreenBg)
    } else {
        arrayOf("⚡", "未启用 - 点击下方按钮开始过滤", Gray700, Gray100)
    }

    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = bgColor as Color)
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text(text = icon as String, fontSize = 24.sp)
            Spacer(modifier = Modifier.width(12.dp))
            Text(
                text = label as String,
                fontSize = 14.sp,
                fontWeight = FontWeight.SemiBold,
                color = color as Color
            )
        }
    }
}

@Composable
private fun GuideStep(
    step: String,
    title: String,
    description: String,
    details: String? = null
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = Color.White),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
    ) {
        Row(modifier = Modifier.padding(16.dp)) {
            Surface(
                shape = androidx.compose.foundation.shape.CircleShape,
                color = Blue500,
                modifier = Modifier.size(32.dp)
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Text(
                        text = step,
                        color = Color.White,
                        fontSize = 14.sp,
                        fontWeight = FontWeight.Bold
                    )
                }
            }

            Spacer(modifier = Modifier.width(12.dp))

            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    fontSize = 15.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = Gray900
                )
                Spacer(modifier = Modifier.height(4.dp))
                Text(
                    text = description,
                    fontSize = 13.sp,
                    color = Gray700
                )

                if (details != null) {
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = details,
                        fontSize = 12.sp,
                        color = Gray500,
                        lineHeight = 18.sp
                    )
                }
            }
        }
    }
}
