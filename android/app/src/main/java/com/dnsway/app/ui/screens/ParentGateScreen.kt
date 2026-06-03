package com.dnsway.app.ui.screens

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.dnsway.app.ui.theme.*
import com.dnsway.app.util.PinManager

/**
 * Full-screen PIN gate that must be passed to access protected features.
 * Shows differently depending on whether PIN has been set up.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ParentGateScreen(
    onVerified: () -> Unit,
    onDismiss: () -> Unit,
    isSetup: Boolean = false // true = first-time PIN setup
) {
    val ctx = LocalContext.current
    var pin by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    var showRecovery by remember { mutableStateOf(false) }
    var recoveryKey by remember { mutableStateOf<String?>(null) }
    var showForgotPin by remember { mutableStateOf(false) }
    var forgotInput by remember { mutableStateOf("") }
    var newPinAfterReset by remember { mutableStateOf("") }
    var step by remember { mutableIntStateOf(0) } // 0=enter/confirm, 1=show recovery

    // First-time setup flow
    if (isSetup) {
        PinSetupContent(
            pin = pin,
            onPinChange = { pin = it },
            error = error,
            onConfirm = {
                if (pin.length < 4) {
                    error = "PIN 至少 4 位"
                    return@PinSetupContent
                }
                val key = PinManager.setPin(ctx, pin)
                recoveryKey = key
                step = 1
                error = null
            },
            step = step,
            recoveryKey = recoveryKey,
            onDone = onVerified
        )
        return
    }

    // Forgot PIN flow
    if (showForgotPin) {
        ForgotPinContent(
            input = forgotInput,
            onInputChange = { forgotInput = it },
            newPin = newPinAfterReset,
            onNewPinChange = { newPinAfterReset = it },
            error = error,
            onVerify = {
                if (PinManager.verifyRecoveryKey(ctx, forgotInput)) {
                    if (newPinAfterReset.length < 4) {
                        error = "新 PIN 至少 4 位"
                        return@ForgotPinContent
                    }
                    PinManager.resetPin(ctx, newPinAfterReset)
                    recoveryKey = PinManager.getRecoveryKey(ctx)
                    step = 1
                    showForgotPin = false
                    error = null
                } else {
                    error = "恢复密钥错误"
                }
            },
            step = step,
            recoveryKey = recoveryKey,
            onDone = { onVerified() },
            onBack = { showForgotPin = false; error = null; forgotInput = ""; newPinAfterReset = "" }
        )
        return
    }

    // Normal PIN verification
    AlertDialog(
        onDismissRequest = onDismiss,
        title = {
            Text(
                "家长验证",
                fontWeight = FontWeight.Bold
            )
        },
        text = {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Text(
                    "请输入家长 PIN 码",
                    fontSize = 14.sp,
                    color = Gray700
                )
                Spacer(modifier = Modifier.height(16.dp))
                OutlinedTextField(
                    value = pin,
                    onValueChange = {
                        pin = it.filter { c -> c.isDigit() }.take(6)
                        error = null
                    },
                    label = { Text("PIN 码") },
                    visualTransformation = PasswordVisualTransformation(),
                    keyboardOptions = KeyboardOptions(
                        keyboardType = KeyboardType.NumberPassword,
                        imeAction = ImeAction.Done
                    ),
                    keyboardActions = KeyboardActions(onDone = {
                        if (PinManager.verifyPin(ctx, pin)) {
                            onVerified()
                        } else {
                            error = "PIN 码错误"
                        }
                    }),
                    singleLine = true,
                    isError = error != null,
                    supportingText = error?.let { { Text(it, color = MaterialTheme.colorScheme.error) } }
                )
                Spacer(modifier = Modifier.height(12.dp))
                TextButton(onClick = { showForgotPin = true; error = null; pin = "" }) {
                    Text("忘记 PIN？", fontSize = 13.sp, color = Blue500)
                }
            }
        },
        confirmButton = {
            Button(
                onClick = {
                    if (PinManager.verifyPin(ctx, pin)) {
                        onVerified()
                    } else {
                        error = "PIN 码错误"
                    }
                },
                enabled = pin.isNotEmpty()
            ) {
                Text("确认")
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text("取消")
            }
        }
    )
}

@Composable
private fun PinSetupContent(
    pin: String,
    onPinChange: (String) -> Unit,
    error: String?,
    onConfirm: () -> Unit,
    step: Int,
    recoveryKey: String?,
    onDone: () -> Unit
) {
    val ctx = LocalContext.current

    AlertDialog(
        onDismissRequest = { /* can't dismiss during setup */ },
        title = {
            Text(
                if (step == 0) "设置家长 PIN" else "恢复密钥",
                fontWeight = FontWeight.Bold
            )
        },
        text = {
            if (step == 0) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(
                        "设置一个 4-6 位数字 PIN，\n孩子修改设置时需要验证。",
                        fontSize = 14.sp,
                        color = Gray700
                    )
                    Spacer(modifier = Modifier.height(16.dp))
                    OutlinedTextField(
                        value = pin,
                        onValueChange = { onPinChange(it.filter { c -> c.isDigit() }.take(6)); Unit },
                        label = { Text("4-6 位数字 PIN") },
                        visualTransformation = PasswordVisualTransformation(),
                        keyboardOptions = KeyboardOptions(
                            keyboardType = KeyboardType.NumberPassword,
                            imeAction = ImeAction.Done
                        ),
                        keyboardActions = KeyboardActions(onDone = { onConfirm() }),
                        singleLine = true,
                        isError = error != null,
                        supportingText = error?.let { { Text(it, color = MaterialTheme.colorScheme.error) } }
                    )
                }
            } else {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(
                        "请立即保存以下恢复密钥！",
                        fontSize = 14.sp,
                        fontWeight = FontWeight.Bold,
                        color = Red500
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(
                        "忘记 PIN 时可用此密钥重置。\n建议截图保存或写在纸上。",
                        fontSize = 13.sp,
                        color = Gray700
                    )
                    Spacer(modifier = Modifier.height(12.dp))
                    Surface(
                        color = MaterialTheme.colorScheme.surfaceVariant,
                        shape = MaterialTheme.shapes.medium,
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Text(
                            text = recoveryKey ?: "",
                            fontSize = 20.sp,
                            fontWeight = FontWeight.Bold,
                            modifier = Modifier.padding(16.dp),
                            letterSpacing = 2.sp
                        )
                    }
                }
            }
        },
        confirmButton = {
            Button(onClick = {
                if (step == 0) onConfirm() else onDone()
            }) {
                Text(if (step == 0) "确认" else "我已保存，开始使用")
            }
        },
        dismissButton = if (step == 0) {
            { TextButton(onClick = { /* can't skip */ }) { Text("跳过") } }
        } else null
    )
}

@Composable
private fun ForgotPinContent(
    input: String,
    onInputChange: (String) -> Unit,
    newPin: String,
    onNewPinChange: (String) -> Unit,
    error: String?,
    onVerify: () -> Unit,
    step: Int,
    recoveryKey: String?,
    onDone: () -> Unit,
    onBack: () -> Unit
) {
    AlertDialog(
        onDismissRequest = onBack,
        title = { Text("重置 PIN", fontWeight = FontWeight.Bold) },
        text = {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                if (step == 0) {
                    Text(
                        "输入设置 PIN 时获得的恢复密钥",
                        fontSize = 14.sp,
                        color = Gray700
                    )
                    Spacer(modifier = Modifier.height(12.dp))
                    OutlinedTextField(
                        value = input,
                        onValueChange = { onInputChange(it.uppercase()); Unit },
                        label = { Text("恢复密钥") },
                        placeholder = { Text("XXXX-XXXX-XXXX-XXXX") },
                        singleLine = true,
                        isError = error != null,
                        supportingText = error?.let { { Text(it, color = MaterialTheme.colorScheme.error) } }
                    )
                    Spacer(modifier = Modifier.height(16.dp))
                    Text(
                        "验证通过后设置新 PIN",
                        fontSize = 13.sp,
                        color = Gray500
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    OutlinedTextField(
                        value = newPin,
                        onValueChange = { onNewPinChange(it.filter { c -> c.isDigit() }.take(6)); Unit },
                        label = { Text("新 PIN 码") },
                        visualTransformation = PasswordVisualTransformation(),
                        keyboardOptions = KeyboardOptions(
                            keyboardType = KeyboardType.NumberPassword
                        ),
                        singleLine = true
                    )
                } else {
                    Text("PIN 已重置！", fontWeight = FontWeight.Bold)
                    Spacer(modifier = Modifier.height(8.dp))
                    Text("新恢复密钥：", fontSize = 14.sp, color = Gray700)
                    Spacer(modifier = Modifier.height(8.dp))
                    Surface(
                        color = MaterialTheme.colorScheme.surfaceVariant,
                        shape = MaterialTheme.shapes.medium
                    ) {
                        Text(
                            text = recoveryKey ?: "",
                            fontSize = 20.sp,
                            fontWeight = FontWeight.Bold,
                            modifier = Modifier.padding(16.dp),
                            letterSpacing = 2.sp
                        )
                    }
                }
            }
        },
        confirmButton = {
            Button(onClick = {
                if (step == 0) onVerify() else onDone()
            }) {
                Text(if (step == 0) "验证并重置" else "完成")
            }
        },
        dismissButton = {
            TextButton(onClick = if (step == 0) onBack else onDone) {
                Text("取消")
            }
        }
    )
}
