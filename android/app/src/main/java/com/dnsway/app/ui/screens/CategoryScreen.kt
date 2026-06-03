package com.dnsway.app.ui.screens

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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.dnsway.app.engine.DnsEngine
import com.dnsway.app.ui.theme.*

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CategoryScreen(onBack: () -> Unit) {
    val categoryStates by DnsEngine.categoryStates.collectAsState()

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("分类管理") },
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
                .padding(16.dp)
        ) {
            Text(
                text = "开启需要拦截的分类",
                fontSize = 14.sp,
                color = Gray700,
                modifier = Modifier.padding(bottom = 16.dp)
            )

            DnsEngine.allCategories.forEach { cat ->
                val enabled = categoryStates[cat.id] ?: true
                CategoryItem(
                    name = cat.name,
                    description = cat.description,
                    emoji = cat.emoji,
                    enabled = enabled,
                    onToggle = { DnsEngine.setCategoryEnabled(cat.id, it) }
                )
                Spacer(modifier = Modifier.height(8.dp))
            }

            Spacer(modifier = Modifier.height(16.dp))

            // Info card
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = BlueBg)
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = "说明",
                        fontSize = 14.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = Gray900
                    )
                    Spacer(modifier = Modifier.height(6.dp))
                    Text(
                        text = "关闭某个分类后，该分类下的域名将不会被拦截。\n" +
                                "内置 270 万+ 域名映射，覆盖 31 个拦截数据源。\n" +
                                "白名单优先级高于分类设置。",
                        fontSize = 13.sp,
                        color = Gray700,
                        lineHeight = 20.sp
                    )
                }
            }
        }
    }
}

@Composable
private fun CategoryItem(
    name: String,
    description: String,
    emoji: String,
    enabled: Boolean,
    onToggle: (Boolean) -> Unit
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = Color.White),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text(text = emoji, fontSize = 24.sp)
            Spacer(modifier = Modifier.width(12.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = name,
                    fontSize = 15.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = Gray900
                )
                Text(
                    text = description,
                    fontSize = 13.sp,
                    color = Gray500
                )
            }
            Switch(
                checked = enabled,
                onCheckedChange = onToggle,
                colors = SwitchDefaults.colors(
                    checkedTrackColor = Green500,
                    checkedThumbColor = Color.White
                )
            )
        }
    }
}
