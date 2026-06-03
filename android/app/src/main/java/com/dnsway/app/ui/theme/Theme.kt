package com.dnsway.app.ui.theme

import android.os.Build
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext

private val LightColorScheme = lightColorScheme(
    primary = Blue500,
    onPrimary = androidx.compose.ui.graphics.Color.White,
    primaryContainer = BlueBg,
    secondary = Green500,
    background = Gray50,
    surface = androidx.compose.ui.graphics.Color.White,
    surfaceVariant = Gray100,
    outline = Gray200,
    error = Red500,
    onBackground = Gray900,
    onSurface = Gray900,
)

@Composable
fun DnswayTheme(
    content: @Composable () -> Unit
) {
    MaterialTheme(
        colorScheme = LightColorScheme,
        typography = Typography(),
        content = content
    )
}
