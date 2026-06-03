# Dnsway ProGuard Rules

# Keep JNI native methods
-keepclasseswithmembernames class com.dnsway.app.engine.DnsEngine {
    native <methods>;
}

# Keep Room entities
-keep class com.dnsway.app.data.models.** { *; }

# Keep OkHttp
-dontwarn okhttp3.**
-dontwarn okio.**
