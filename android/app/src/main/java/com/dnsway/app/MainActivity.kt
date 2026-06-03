package com.dnsway.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Security
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.dnsway.app.ui.screens.*
import com.dnsway.app.ui.theme.DnswayTheme
import com.dnsway.app.util.PinManager

sealed class BottomNavItem(val route: String, val label: String, val icon: ImageVector) {
    data object Home : BottomNavItem("home", "首页", Icons.Default.Home)
    data object Rules : BottomNavItem("rules", "规则", Icons.Default.Security)
    data object Settings : BottomNavItem("settings", "设置", Icons.Default.Settings)
}

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            DnswayTheme {
                MainScreen()
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MainScreen() {
    val navController = rememberNavController()
    val items = listOf(BottomNavItem.Home, BottomNavItem.Rules, BottomNavItem.Settings)
    val ctx = LocalContext.current

    // PIN gate state: null = no gate, otherwise stores pending action description
    var pendingPinAction by remember { mutableStateOf<String?>(null) }
    var showPinGate by remember { mutableStateOf(false) }
    var showPinSetup by remember { mutableStateOf(false) }
    var firstLaunch by remember { mutableStateOf(!PinManager.isPinSet(ctx)) }

    // Show PIN setup on first launch
    LaunchedEffect(firstLaunch) {
        if (!PinManager.isPinSet(ctx)) {
            showPinSetup = true
        }
    }

    // PIN gate dialog — shown before protected actions
    if (showPinGate) {
        ParentGateScreen(
            onVerified = {
                showPinGate = false
                pendingPinAction = null
            },
            onDismiss = {
                showPinGate = false
                pendingPinAction = null
            }
        )
    }

    // First-launch PIN setup
    if (showPinSetup) {
        ParentGateScreen(
            onVerified = {
                showPinSetup = false
                firstLaunch = false
            },
            onDismiss = { /* can't skip */ },
            isSetup = true
        )
    }

    Scaffold(
        bottomBar = {
            NavigationBar {
                val navBackStackEntry by navController.currentBackStackEntryAsState()
                val currentDestination = navBackStackEntry?.destination

                items.forEach { item ->
                    val isProtected = item.route == "rules" || item.route == "settings"

                    NavigationBarItem(
                        icon = { Icon(item.icon, contentDescription = item.label) },
                        label = { Text(item.label) },
                        selected = currentDestination?.hierarchy?.any { it.route == item.route } == true,
                        onClick = {
                            if (isProtected && PinManager.isPinSet(ctx)) {
                                showPinGate = true
                                pendingPinAction = item.route
                            } else {
                                navController.navigate(item.route) {
                                    popUpTo(navController.graph.findStartDestination().id) {
                                        saveState = true
                                    }
                                    launchSingleTop = true
                                    restoreState = true
                                }
                            }
                        }
                    )
                }
            }
        }
    ) { paddingValues ->
        NavHost(
            navController = navController,
            startDestination = BottomNavItem.Home.route,
            modifier = Modifier.padding(paddingValues)
        ) {
            composable("home") {
                HomeScreen(
                    onNavigateToGuide = { navController.navigate("guide") },
                    onNavigateToLogs = { navController.navigate("logs") },
                    onRequirePin = {
                        if (PinManager.isPinSet(ctx)) {
                            showPinGate = true
                        } else {
                            it()
                        }
                    }
                )
            }
            composable("rules") {
                RulesScreen()
            }
            composable("settings") {
                SettingsScreen(
                    onNavigateToCategories = {
                        if (PinManager.isPinSet(ctx)) {
                            showPinGate = true
                            pendingPinAction = "categories"
                        } else {
                            navController.navigate("categories")
                        }
                    },
                    onNavigateToLogs = { navController.navigate("logs") },
                    onNavigateToSecurity = {
                        if (PinManager.isPinSet(ctx)) {
                            showPinGate = true
                            pendingPinAction = "security"
                        } else {
                            navController.navigate("security")
                        }
                    },
                    onNavigateToPrivacy = { navController.navigate("privacy") },
                    onRequirePin = {
                        if (PinManager.isPinSet(ctx)) {
                            showPinGate = true
                        } else {
                            it()
                        }
                    }
                )
            }
            composable("categories") {
                CategoryScreen(onBack = { navController.popBackStack() })
            }
            composable("logs") {
                QueryLogScreen(onBack = { navController.popBackStack() })
            }
            composable("guide") {
                DnsGuideScreen(
                    onBack = { navController.popBackStack() },
                    onNavigateToSecurity = { navController.navigate("security") }
                )
            }
            composable("security") {
                SecuritySetupScreen(onBack = { navController.popBackStack() })
            }
            composable("privacy") {
                PrivacyPolicyScreen(onBack = { navController.popBackStack() })
            }
        }
    }

    // Handle pending navigation after PIN verified
    LaunchedEffect(showPinGate) {
        if (!showPinGate && pendingPinAction != null) {
            val route = pendingPinAction!!
            if (route in listOf("home", "rules", "settings")) {
                navController.navigate(route) {
                    popUpTo(navController.graph.findStartDestination().id) {
                        saveState = true
                    }
                    launchSingleTop = true
                    restoreState = true
                }
            } else if (route.isNotEmpty()) {
                navController.navigate(route)
            }
            pendingPinAction = null
        }
    }
}
