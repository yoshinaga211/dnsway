package com.dnsway.app.ui.screens

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.dnsway.app.DnswayApp
import com.dnsway.app.data.models.Rule
import com.dnsway.app.ui.theme.*
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RulesScreen() {
    val dao = DnswayApp.instance.database.ruleDao()
    val scope = rememberCoroutineScope()

    val allowlist by dao.getAllAllowlist().collectAsState(initial = emptyList())
    val denylist by dao.getAllDenylist().collectAsState(initial = emptyList())

    var selectedTab by remember { mutableIntStateOf(0) }
    var showAddDialog by remember { mutableStateOf(false) }
    var addType by remember { mutableStateOf("allow") }

    Box(modifier = Modifier.fillMaxSize()) {
        Column(modifier = Modifier.fillMaxSize()) {
            TabRow(selectedTabIndex = selectedTab) {
                Tab(selected = selectedTab == 0, onClick = { selectedTab = 0 }) {
                    Text("白名单 (${allowlist.size})", modifier = Modifier.padding(16.dp))
                }
                Tab(selected = selectedTab == 1, onClick = { selectedTab = 1 }) {
                    Text("黑名单 (${denylist.size})", modifier = Modifier.padding(16.dp))
                }
            }

            val currentList = if (selectedTab == 0) allowlist else denylist

            if (currentList.isEmpty()) {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center
                ) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(
                            text = if (selectedTab == 0) "暂无白名单规则" else "暂无黑名单规则",
                            fontSize = 16.sp,
                            color = Gray500
                        )
                        Spacer(modifier = Modifier.height(8.dp))
                        Text(
                            text = if (selectedTab == 0) "添加域名到白名单以始终允许访问" else "添加域名到黑名单以始终拦截",
                            fontSize = 13.sp,
                            color = Gray300
                        )
                    }
                }
            } else {
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(start = 8.dp, end = 8.dp, top = 8.dp, bottom = 72.dp)
                ) {
                    items(currentList, key = { it.id }) { rule ->
                        RuleItem(rule = rule, onDelete = {
                            scope.launch { dao.delete(rule) }
                        })
                    }
                }
            }
        }

        // FAB
        FloatingActionButton(
            onClick = {
                addType = if (selectedTab == 0) "allow" else "block"
                showAddDialog = true
            },
            modifier = Modifier
                .align(Alignment.BottomEnd)
                .padding(16.dp),
            containerColor = Blue500
        ) {
            Icon(Icons.Default.Add, contentDescription = "添加", tint = androidx.compose.ui.graphics.Color.White)
        }

        if (showAddDialog) {
            AddRuleDialog(
                type = addType,
                onDismiss = { showAddDialog = false },
                onConfirm = { domain ->
                    scope.launch {
                        dao.insert(
                            Rule(
                                domain = domain.trim().lowercase(),
                                type = addType,
                                reason = ""
                            )
                        )
                    }
                    showAddDialog = false
                }
            )
        }
    }
}

@Composable
fun RuleItem(rule: Rule, onDelete: () -> Unit) {
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp),
        shape = RoundedCornerShape(10.dp),
        colors = CardDefaults.cardColors(containerColor = androidx.compose.ui.graphics.Color.White),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = rule.domain,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.Medium,
                    color = Gray900
                )
                if (rule.reason.isNotEmpty()) {
                    Text(
                        text = rule.reason,
                        fontSize = 12.sp,
                        color = Gray500
                    )
                }
            }
            IconButton(onClick = onDelete) {
                Icon(Icons.Default.Delete, contentDescription = "删除", tint = Red500)
            }
        }
    }
}

@Composable
fun AddRuleDialog(type: String, onDismiss: () -> Unit, onConfirm: (String) -> Unit) {
    var domain by remember { mutableStateOf("") }
    var reason by remember { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = {
            Text(
                text = if (type == "allow") "添加白名单" else "添加黑名单",
                fontWeight = FontWeight.Bold
            )
        },
        text = {
            Column {
                OutlinedTextField(
                    value = domain,
                    onValueChange = { domain = it },
                    label = { Text("域名") },
                    placeholder = { Text("example.com") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
                Spacer(modifier = Modifier.height(8.dp))
                OutlinedTextField(
                    value = reason,
                    onValueChange = { reason = it },
                    label = { Text("原因（可选）") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
            }
        },
        confirmButton = {
            TextButton(
                onClick = {
                    if (domain.isNotBlank()) {
                        onConfirm(domain.trim().lowercase())
                    }
                },
                enabled = domain.isNotBlank()
            ) {
                Text("添加")
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text("取消")
            }
        }
    )
}
