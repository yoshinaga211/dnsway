package com.dnsway.app.data

import androidx.room.*
import com.dnsway.app.data.models.*
import kotlinx.coroutines.flow.Flow

@Dao
interface RuleDao {

    @Query("SELECT * FROM rules WHERE type = 'allow' ORDER BY domain ASC")
    fun getAllAllowlist(): Flow<List<Rule>>

    @Query("SELECT * FROM rules WHERE type = 'block' ORDER BY domain ASC")
    fun getAllDenylist(): Flow<List<Rule>>

    @Query("SELECT * FROM rules WHERE domain = :domain AND type = :type LIMIT 1")
    suspend fun findByDomain(domain: String, type: String): Rule?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insert(rule: Rule): Long

    @Delete
    suspend fun delete(rule: Rule)

    @Query("DELETE FROM rules WHERE domain = :domain AND type = :type")
    suspend fun deleteByDomain(domain: String, type: String)

    @Query("SELECT * FROM query_logs ORDER BY timestamp DESC LIMIT 100")
    fun getRecentLogs(): Flow<List<QueryLog>>

    @Insert
    suspend fun insertLog(log: QueryLog)

    @Query("SELECT COUNT(*) FROM query_logs WHERE timestamp >= :since")
    fun totalQueriesSince(since: Long): Flow<Int>

    @Query("SELECT COUNT(*) FROM query_logs WHERE decision = 'BLOCK' AND timestamp >= :since")
    fun blockedQueriesSince(since: Long): Flow<Int>

    @Query("DELETE FROM query_logs WHERE timestamp < :before")
    suspend fun cleanOldLogs(before: Long)

    // Category configs
    @Query("SELECT * FROM category_configs ORDER BY categoryId ASC")
    fun getAllCategories(): Flow<List<CategoryConfig>>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertCategory(config: CategoryConfig)

    @Query("UPDATE category_configs SET enabled = :enabled WHERE categoryId = :id")
    suspend fun setCategoryEnabled(id: String, enabled: Boolean)

    @Query("SELECT enabled FROM category_configs WHERE categoryId = :id")
    suspend fun isCategoryEnabled(id: String): Boolean?

    @Query("DELETE FROM query_logs")
    suspend fun clearLogs()
}
