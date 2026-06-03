package com.dnsway.app.ui.screens

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.dnsway.app.DnswayApp
import com.dnsway.app.data.models.QueryLog
import com.dnsway.app.ui.theme.*
import java.text.SimpleDateFormat
import java.util.*

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun QueryLogScreen(onBack: () -> Unit) {
    val dao = DnswayApp.instance.database.ruleDao()
    val logs by dao.getRecentLogs().collectAsState(initial = emptyList())

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("查询日志") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                }
            )
        }
    ) { padding ->
        if (logs.isEmpty()) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding),
                contentAlignment = Alignment.Center
            ) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text("暂无查询记录", fontSize = 16.sp, color = Gray500)
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(
                        text = "启动 VPN 后，DNS 查询会显示在这里",
                        fontSize = 13.sp,
                        color = Gray300
                    )
                }
            }
        } else {
            LazyColumn(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding),
                contentPadding = PaddingValues(12.dp),
                verticalArrangement = Arrangement.spacedBy(6.dp)
            ) {
                items(logs, key = { it.id }) { log ->
                    QueryLogItem(log = log)
                }
            }
        }
    }
}

@Composable
private fun QueryLogItem(log: QueryLog) {
    val isBlocked = log.decision == "BLOCK"
    val timeStr = remember(log.timestamp) {
        val sdf = SimpleDateFormat("HH:mm:ss", Locale.getDefault())
        sdf.format(Date(log.timestamp))
    }

    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(10.dp),
        colors = CardDefaults.cardColors(containerColor = Color.White),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 14.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            // Status indicator
            Surface(
                shape = RoundedCornerShape(6.dp),
                color = if (isBlocked) RedBg else GreenBg,
                modifier = Modifier.size(36.dp)
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Text(
                        text = if (isBlocked) "拦" else "放",
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Bold,
                        color = if (isBlocked) Red500 else Green500
                    )
                }
            }

            Spacer(modifier = Modifier.width(10.dp))

            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = log.domain,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.Medium,
                    color = Gray900,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (log.reason.isNotEmpty()) {
                    Text(
                        text = log.reason,
                        fontSize = 11.sp,
                        color = Gray500,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }

            Text(
                text = timeStr,
                fontSize = 11.sp,
                color = Gray300
            )
        }
    }
}
