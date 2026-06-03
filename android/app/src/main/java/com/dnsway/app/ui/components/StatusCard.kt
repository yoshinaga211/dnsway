package com.dnsway.app.ui.components

import androidx.compose.animation.core.*
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.dnsway.app.network.ProtectionLevel
import com.dnsway.app.ui.theme.*

@Composable
fun StatusCard(
    level: ProtectionLevel,
    latencyMs: Long,
    onClick: () -> Unit
) {
    val (icon, label, color, bgColor) = when (level) {
        ProtectionLevel.PROTECTED -> StatusCardData("🛡️", "已保护", Green500, GreenBg)
        ProtectionLevel.NOT_CONFIGURED -> StatusCardData("⚠️", "未配置", Orange500, OrangeBg)
        ProtectionLevel.BYPASSED -> StatusCardData("❌", "已绕过", Red500, RedBg)
        ProtectionLevel.ERROR -> StatusCardData("🔌", "检测失败", Gray500, Gray100)
    }

    Card(
        onClick = onClick,
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(containerColor = Color.White),
        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp)
    ) {
        Column(modifier = Modifier.padding(20.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(
                    modifier = Modifier
                        .size(48.dp)
                        .clip(CircleShape)
                        .background(bgColor),
                    contentAlignment = Alignment.Center
                ) {
                    Text(text = icon, fontSize = 22.sp)
                }

                Spacer(modifier = Modifier.width(14.dp))

                Column {
                    Text(
                        text = "DNS 状态",
                        fontSize = 13.sp,
                        color = Gray500
                    )
                    Spacer(modifier = Modifier.height(2.dp))
                    Text(
                        text = label,
                        fontSize = 18.sp,
                        fontWeight = FontWeight.Bold,
                        color = color
                    )
                }
            }

            if (latencyMs > 0) {
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = "延迟: ${latencyMs}ms",
                    fontSize = 12.sp,
                    color = Gray500
                )
            }
        }
    }
}

private data class StatusCardData(
    val icon: String,
    val label: String,
    val color: Color,
    val bgColor: Color
)

@Composable
fun StatsCard(
    totalQueries: Int,
    blockedCount: Int,
    modifier: Modifier = Modifier
) {
    Row(
        modifier = modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        StatItem(
            value = totalQueries.toString(),
            label = "总查询",
            color = Blue500,
            modifier = Modifier.weight(1f)
        )
        StatItem(
            value = blockedCount.toString(),
            label = "已拦截",
            color = Red500,
            modifier = Modifier.weight(1f)
        )
    }
}

@Composable
fun StatItem(
    value: String,
    label: String,
    color: Color,
    modifier: Modifier = Modifier
) {
    Card(
        modifier = modifier,
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(containerColor = Color.White),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
    ) {
        Column(
            modifier = Modifier.padding(16.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Text(
                text = value,
                fontSize = 28.sp,
                fontWeight = FontWeight.ExtraBold,
                color = color
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = label,
                fontSize = 13.sp,
                color = Gray500
            )
        }
    }
}

@Composable
fun QuickActionButton(
    text: String,
    modifier: Modifier = Modifier,
    onClick: () -> Unit
) {
    Button(
        onClick = onClick,
        modifier = modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        colors = ButtonDefaults.buttonColors(containerColor = Blue500)
    ) {
        Text(text = text, fontSize = 15.sp, fontWeight = FontWeight.SemiBold)
    }
}
