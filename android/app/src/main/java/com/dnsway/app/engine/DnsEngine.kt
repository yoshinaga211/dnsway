package com.dnsway.app.engine

import android.content.Context
import android.util.Log
import com.dnsway.app.DnswayApp
import com.dnsway.app.data.AppDatabase
import com.dnsway.app.data.models.CategoryConfig
import com.dnsway.app.data.models.DailyStats
import com.dnsway.app.data.models.QueryLog
import java.io.File
import java.io.FileOutputStream
import java.util.concurrent.atomic.AtomicBoolean
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.runBlocking

/**
 * JNI bridge to the Go filtering engine (libdnsway.so).
 *
 * The Go engine is compiled as a c-shared library for arm64.
 * It handles the core domain decision logic:
 *   allowlist → denylist → category match → default allow
 */
object DnsEngine {

    private const val TAG = "DnsEngine"
    private val initialized = AtomicBoolean(false)
    val isInitialized: Boolean get() = initialized.get()
    private val statsLock = Any()
    private var todayTotal = 0
    private var todayBlocked = 0

    // ── Category Definitions ──

    data class CategoryInfo(
        val id: String,
        val name: String,
        val description: String,
        val emoji: String,
        val enabled: Boolean = true,
        val domainCount: Int = 0
    )

    val allCategories = listOf(
        CategoryInfo("CAT_001", "成人内容", "色情、成人网站等", "🔞"),
        CategoryInfo("CAT_005", "游戏", "网络游戏、游戏平台", "🎮"),
    )

    private val _categoryStates = MutableStateFlow(
        allCategories.associate { it.id to true }
    )
    val categoryStates: StateFlow<Map<String, Boolean>> = _categoryStates

    fun getCategoryEnabled(id: String): Boolean = _categoryStates.value[id] ?: true

    fun setCategoryEnabled(id: String, enabled: Boolean): Boolean {
        _categoryStates.value = _categoryStates.value.toMutableMap().also { it[id] = enabled }
        return nativeSetCategory(id, enabled)
    }

    // ── Safe Search ──

    private val _safeSearchEnabled = MutableStateFlow(false)
    val safeSearchEnabled: StateFlow<Boolean> = _safeSearchEnabled

    fun setSafeSearchEnabled(enabled: Boolean) {
        _safeSearchEnabled.value = enabled
        nativeSetSafeSearch(enabled)
    }

    // ── JNI native methods (implemented in Go) ──

    /** Initialize engine with path to domain data directory */
    private external fun nativeInit(dataDir: String): Boolean

    /** Process a domain: returns "ALLOW", "BLOCK:reason", or "PASS" */
    private external fun nativeProcessDomain(domain: String): String

    /** Add domain to allowlist */
    external fun nativeAddAllowlist(domain: String): Boolean

    /** Remove domain from allowlist */
    external fun nativeRemoveAllowlist(domain: String): Boolean

    /** Add domain to denylist */
    external fun nativeAddDenylist(domain: String, reason: String): Boolean

    /** Remove domain from denylist */
    external fun nativeRemoveDenylist(domain: String): Boolean

    /** Enable/disable a category */
    external fun nativeSetCategory(categoryId: String, enabled: Boolean): Boolean

    /** Get engine stats as JSON */
    external fun nativeGetStats(): String

    /** Load domain-category mappings from a file */
    external fun nativeLoadCategories(path: String): Int

    /** Check if safe search should be enforced */
    external fun nativeShouldSafeSearch(domain: String): Boolean

    /** Set safe search enforcement */
    external fun nativeSetSafeSearch(enabled: Boolean)

    // ── Initialization ──

    fun initialize(context: Context): Boolean {
        if (initialized.get()) return true

        return try {
            System.loadLibrary("dnsway")
            val dataDir = extractAssets(context)
            val ok = nativeInit(dataDir)
            if (ok) {
                loadCategoryData(dataDir)
                initialized.set(true)
                Log.i(TAG, "Engine initialized successfully")
            }
            ok
        } catch (e: UnsatisfiedLinkError) {
            Log.w(TAG, "Native library not available, using local-only mode", e)
            initialized.set(true)
            true
        }
    }

    private fun extractAssets(context: Context): String {
        val dir = File(context.filesDir, "engine_data").also { it.mkdirs() }

        // Extract bundled domain lists if they exist
        try {
            context.assets.list("categories")?.forEach { name ->
                val out = File(dir, name)
                if (!out.exists()) {
                    context.assets.open("categories/$name").use { input ->
                        FileOutputStream(out).use { output ->
                            input.copyTo(output)
                        }
                    }
                }
            }
        } catch (e: Exception) {
            Log.w(TAG, "No bundled categories in assets", e)
        }

        return dir.absolutePath
    }

    private fun loadCategoryData(dataDir: String): Int {
        val file = File(dataDir, "domain_categories.csv")
        if (file.exists()) {
            return nativeLoadCategories(file.absolutePath)
        }
        return 0
    }

    // ── Domain Processing ──

    fun processDomain(domain: String): Decision {
        if (!initialized.get()) return Decision.ALLOW

        return try {
            val result = nativeProcessDomain(domain)
            Decision.fromNative(result)
        } catch (e: Exception) {
            Log.e(TAG, "processDomain error: ${e.message}")
            Decision.ALLOW
        }
    }

    // ─── Allowlist / Denylist ──

    fun addAllowlist(domain: String): Boolean = nativeAddAllowlist(domain.trim().lowercase())
    fun removeAllowlist(domain: String): Boolean = nativeRemoveAllowlist(domain.trim().lowercase())
    fun addDenylist(domain: String, reason: String = ""): Boolean = nativeAddDenylist(domain.trim().lowercase(), reason)
    fun removeDenylist(domain: String): Boolean = nativeRemoveDenylist(domain.trim().lowercase())

    // ─── Category Toggles ──

    fun setCategory(categoryId: String, enabled: Boolean): Boolean {
        return nativeSetCategory(categoryId, enabled)
    }

    // ─── Logging ──

    fun recordQuery(domain: String, decision: Decision) {
        synchronized(statsLock) {
            todayTotal++
            if (decision == Decision.BLOCK) todayBlocked++
        }

        // Write to Room asynchronously
        try {
            val db = AppDatabase.getInstance(DnswayApp.instance)
            Thread {
                runBlocking {
                    db.ruleDao().insertLog(
                        QueryLog(
                            domain = domain,
                            decision = decision.name,
                            reason = decision.reason
                        )
                    )
                    // Clean old logs (keep 7 days)
                    db.ruleDao().cleanOldLogs(System.currentTimeMillis() - 7 * 86400000L)
                }
            }.start()
        } catch (e: Exception) {
            // Silently handle DB errors
        }
    }

    fun getDailyStats(): DailyStats = synchronized(statsLock) {
        DailyStats(todayTotal, todayBlocked)
    }

    fun resetDailyStats() {
        synchronized(statsLock) {
            todayTotal = 0
            todayBlocked = 0
        }
    }
}

enum class Decision(val native: String, val isBlock: Boolean) {
    ALLOW("ALLOW", false),
    BLOCK("BLOCK", true),
    PASS("PASS", false);

    var reason: String = ""

    companion object {
        fun fromNative(result: String): Decision {
            val clean = result.trim()
            return when {
                clean == "ALLOW" -> ALLOW
                clean == "PASS" -> ALLOW
                clean.startsWith("BLOCK") -> {
                    val colon = clean.indexOf(':')
                    BLOCK.also { it.reason = if (colon > 0) clean.substring(colon + 1) else "" }
                }
                else -> ALLOW
            }
        }
    }
}
