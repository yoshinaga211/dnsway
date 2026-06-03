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
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.dnsway.app.ui.screens.CategoryScreen
import com.dnsway.app.ui.screens.DnsGuideScreen
import com.dnsway.app.ui.screens.HomeScreen
import com.dnsway.app.ui.screens.PrivacyPolicyScreen
import com.dnsway.app.ui.screens.QueryLogScreen
import com.dnsway.app.ui.screens.RulesScreen
import com.dnsway.app.ui.screens.SecuritySetupScreen
import com.dnsway.app.ui.screens.SettingsScreen
import com.dnsway.app.ui.theme.DnswayTheme

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

    Scaffold(
        bottomBar = {
            NavigationBar {
                val navBackStackEntry by navController.currentBackStackEntryAsState()
                val currentDestination = navBackStackEntry?.destination

                items.forEach { item ->
                    NavigationBarItem(
                        icon = { Icon(item.icon, contentDescription = item.label) },
                        label = { Text(item.label) },
                        selected = currentDestination?.hierarchy?.any { it.route == item.route } == true,
                        onClick = {
                            navController.navigate(item.route) {
                                popUpTo(navController.graph.findStartDestination().id) {
                                    saveState = true
                                }
                                launchSingleTop = true
                                restoreState = true
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
                    onNavigateToLogs = { navController.navigate("logs") }
                )
            }
            composable("rules") {
                RulesScreen()
            }
            composable("settings") {
                SettingsScreen(
                    onNavigateToCategories = { navController.navigate("categories") },
                    onNavigateToLogs = { navController.navigate("logs") },
                    onNavigateToSecurity = { navController.navigate("security") },
                    onNavigateToPrivacy = { navController.navigate("privacy") }
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
}
