package com.dnsway.app.data.models

import androidx.room.*

@Entity(tableName = "rules")
data class Rule(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    @ColumnInfo(name = "domain") val domain: String,
    @ColumnInfo(name = "type") val type: String,  // "allow" or "block"
    @ColumnInfo(name = "reason") val reason: String = "",
    @ColumnInfo(name = "created_at") val createdAt: Long = System.currentTimeMillis()
)

@Entity(tableName = "category_configs")
data class CategoryConfig(
    @PrimaryKey val categoryId: String,
    @ColumnInfo(name = "enabled") val enabled: Boolean = true,
    @ColumnInfo(name = "name") val name: String = "",
    @ColumnInfo(name = "domain_count") val domainCount: Int = 0
)

@Entity(tableName = "query_logs")
data class QueryLog(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    @ColumnInfo(name = "domain") val domain: String,
    @ColumnInfo(name = "decision") val decision: String,
    @ColumnInfo(name = "reason") val reason: String = "",
    @ColumnInfo(name = "timestamp") val timestamp: Long = System.currentTimeMillis()
)

data class DailyStats(
    val totalQueries: Int = 0,
    val blockedQueries: Int = 0
)
