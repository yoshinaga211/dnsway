package com.dnsway.app.util

import android.content.Context
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import java.security.SecureRandom

/**
 * Manages parent PIN authentication and recovery key.
 *
 * PIN is stored as SHA-256 hash in EncryptedSharedPreferences.
 * Recovery key is a 16-char alphanumeric code generated during PIN setup.
 * Parent should save the recovery key offline (screenshot / written down).
 */
object PinManager {

    private const val PREFS_NAME = "dnsway_pin_prefs"
    private const val KEY_PIN_HASH = "pin_hash"
    private const val KEY_RECOVERY_HASH = "recovery_hash"
    private const val KEY_RECOVERY_PLAIN = "recovery_plain" // shown once at setup
    private const val SALT = "DnswayPin2026" // static salt

    private var prefs: android.content.SharedPreferences? = null

    private fun prefs(ctx: Context): android.content.SharedPreferences {
        if (prefs == null) {
            val masterKey = MasterKey.Builder(ctx)
                .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
                .build()
            prefs = EncryptedSharedPreferences.create(
                ctx,
                PREFS_NAME,
                masterKey,
                EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
            )
        }
        return prefs!!
    }

    /** Check if a PIN has been set up. */
    fun isPinSet(ctx: Context): Boolean {
        return prefs(ctx).contains(KEY_PIN_HASH)
    }

    /** Set a new PIN. Returns the recovery key. */
    fun setPin(ctx: Context, pin: String): String {
        val recoveryKey = generateRecoveryKey()
        prefs(ctx).edit()
            .putString(KEY_PIN_HASH, hash(pin))
            .putString(KEY_RECOVERY_HASH, hash(recoveryKey))
            .putString(KEY_RECOVERY_PLAIN, recoveryKey)
            .apply()
        return recoveryKey
    }

    /** Verify the entered PIN. */
    fun verifyPin(ctx: Context, pin: String): Boolean {
        val storedHash = prefs(ctx).getString(KEY_PIN_HASH, null) ?: return false
        return hash(pin) == storedHash
    }

    /** Verify a recovery key. */
    fun verifyRecoveryKey(ctx: Context, key: String): Boolean {
        val storedHash = prefs(ctx).getString(KEY_RECOVERY_HASH, null) ?: return false
        return hash(key.trim().uppercase()) == storedHash
    }

    /** Reset PIN using recovery key. */
    fun resetPin(ctx: Context, newPin: String) {
        setPin(ctx, newPin)
    }

    /** Get the plaintext recovery key (shown once at setup, persists for reference). */
    fun getRecoveryKey(ctx: Context): String? {
        return prefs(ctx).getString(KEY_RECOVERY_PLAIN, null)
    }

    /** Remove all PIN data. */
    fun clearPin(ctx: Context) {
        prefs(ctx).edit().clear().apply()
    }

    private fun hash(input: String): String {
        val salted = input + SALT
        val digest = java.security.MessageDigest.getInstance("SHA-256")
        return digest.digest(salted.toByteArray()).joinToString("") { "%02x".format(it) }
    }

    private fun generateRecoveryKey(): String {
        val chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no I/O/0/1 for readability
        val random = SecureRandom()
        return (1..16).map { chars[random.nextInt(chars.length)] }.joinToString("")
            .chunked(4).joinToString("-") // XXXX-XXXX-XXXX-XXXX
    }
}
