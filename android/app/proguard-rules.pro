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

# dnsjava — missing class warnings on Android
-dontwarn org.slf4j.impl.**
-dontwarn sun.net.spi.nameservice.**
-dontwarn org.xbill.DNS.spi.**
-dontwarn com.sun.jna.**
-dontwarn javax.naming.**
-keep class org.xbill.DNS.** { *; }

# R8 compilation mode: don't fail on missing referenced classes
-ignorewarnings
