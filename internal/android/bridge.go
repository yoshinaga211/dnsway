//go:build android

package main

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>

// JNI wrapper helpers (JNI functions must be called through the env function table)
static const char* jni_get_string_utf_chars(JNIEnv* env, jstring js, jboolean* isCopy) {
    return (*env)->GetStringUTFChars(env, js, isCopy);
}

static void jni_release_string_utf_chars(JNIEnv* env, jstring js, const char* chars) {
    (*env)->ReleaseStringUTFChars(env, js, chars);
}

static jstring jni_new_string_utf(JNIEnv* env, const char* chars) {
    return (*env)->NewStringUTF(env, chars);
}

static jboolean jni_get_boolean_field(JNIEnv* env, jobject obj, jfieldID field) {
    return (*env)->GetBooleanField(env, obj, field);
}

static jfieldID jni_get_field_id(JNIEnv* env, jclass clazz, const char* name, const char* sig) {
    return (*env)->GetFieldID(env, clazz, name, sig);
}

static jclass jni_find_class(JNIEnv* env, const char* name) {
    return (*env)->FindClass(env, name);
}

static void jni_call_void_method(JNIEnv* env, jobject obj, jmethodID method, ...) {
    va_list args;
    va_start(args, method);
    (*env)->CallVoidMethodV(env, obj, method, args);
    va_end(args);
}

static jmethodID jni_get_method_id(JNIEnv* env, jclass clazz, const char* name, const char* sig) {
    return (*env)->GetMethodID(env, clazz, name, sig);
}

static jobject jni_get_object_field(JNIEnv* env, jobject obj, jfieldID field) {
    return (*env)->GetObjectField(env, obj, field);
}

// Direct JNI call wrappers for the functions used by the Go code
static const char* call_GetStringUTFChars(JNIEnv* env, jstring js) {
    return (*env)->GetStringUTFChars(env, js, NULL);
}

static void call_ReleaseStringUTFChars(JNIEnv* env, jstring js, const char* s) {
    (*env)->ReleaseStringUTFChars(env, js, s);
}

static jstring call_NewStringUTF(JNIEnv* env, const char* s) {
    return (*env)->NewStringUTF(env, s);
}
*/
import "C"
import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

// ── Engine State ──

var (
	mu             sync.RWMutex
	categoryDB     = make(map[string][]string) // domain → category IDs
	allowlist      = make(map[string]bool)
	denylist       = make(map[string]bool)
		categoryConfig = map[string]bool{
			"CAT_001": true, // Adult content
			"CAT_005": true, // Gaming
		}
	safeSearchEnabled bool
	initialized       int32 // atomic boolean (0/1)

	totalQueries int32
	blockedCount int32
)

// Built-in category mappings — domains blocked by category out of the box.
// Loaded on init; additional mappings can be loaded from CSV via nativeLoadCategories.
var builtinCategoryDB = map[string][]string{
	// ── CAT_001: Adult content ──

	// Major tube sites
	"pornhub.com":      {"CAT_001"},
	"xvideos.com":      {"CAT_001"},
	"xnxx.com":         {"CAT_001"},
	"xhamster.com":     {"CAT_001"},
	"redtube.com":      {"CAT_001"},
	"youporn.com":      {"CAT_001"},
	"tube8.com":        {"CAT_001"},
	"spankbang.com":    {"CAT_001"},
	"beeg.com":         {"CAT_001"},
	"tnaflix.com":      {"CAT_001"},
	"empflix.com":      {"CAT_001"},
	"porntube.com":     {"CAT_001"},
	"4tube.com":        {"CAT_001"},
	"vporn.com":        {"CAT_001"},
	"keezmovies.com":   {"CAT_001"},
	"eporner.com":      {"CAT_001"},
	"porn.com":         {"CAT_001"},

	// Premium studios
	"brazzers.com":         {"CAT_001"},
	"bangbros.com":         {"CAT_001"},
	"realitykings.com":     {"CAT_001"},
	"naughtyamerica.com":   {"CAT_001"},
	"evilangel.com":        {"CAT_001"},
	"vixen.com":            {"CAT_001"},
	"blacked.com":          {"CAT_001"},
	"blackedraw.com":       {"CAT_001"},
	"tushy.com":            {"CAT_001"},
	"team-skeet.com":       {"CAT_001"},
	"girlsway.com":         {"CAT_001"},
	"mofos.com":            {"CAT_001"},
	"digitalplayground.com": {"CAT_001"},
	"twistys.com":          {"CAT_001"},
	"propertysex.com":      {"CAT_001"},
	"wankz.com":            {"CAT_001"},
	"pornfidelity.com":     {"CAT_001"},

	// Live cams
	"chaturbate.com":   {"CAT_001"},
	"stripchat.com":    {"CAT_001"},
	"livejasmin.com":   {"CAT_001"},
	"myfreecams.com":   {"CAT_001"},
	"cam4.com":         {"CAT_001"},
	"bongacams.com":    {"CAT_001"},
	"flirt4free.com":   {"CAT_001"},
	"imlive.com":       {"CAT_001"},
	"streamate.com":    {"CAT_001"},
	"cams.com":         {"CAT_001"},

	// Hentai / adult anime
	"nhentai.net":     {"CAT_001"},
	"hentaihaven.xxx": {"CAT_001"},
	"hentaigasm.com":  {"CAT_001"},
	"hentaifox.com":   {"CAT_001"},
	"pururin.com":     {"CAT_001"},
	"hentai2read.com": {"CAT_001"},
	"hanime.tv":       {"CAT_001"},
	"hentaimama.io":   {"CAT_001"},

	// Adult social / fan sites
	"onlyfans.com":    {"CAT_001"},
	"fansly.com":      {"CAT_001"},
	"manyvids.com":    {"CAT_001"},
	"iwantclips.com":  {"CAT_001"},
	"clipfans.com":    {"CAT_001"},

	// Erotica / stories
	"literotica.com":  {"CAT_001"},
	"lushstories.com": {"CAT_001"},
	"sexstories.com":  {"CAT_001"},
	"asstr.org":       {"CAT_001"},

	// Adult dating / hookup
	"adultfriendfinder.com": {"CAT_001"},
	"fling.com":            {"CAT_001"},
	"ashleymadison.com":    {"CAT_001"},
	"seeking.com":          {"CAT_001"},
	"alt.com":              {"CAT_001"},

	// Adult games
	"nutaku.com":      {"CAT_001"},
	"hentaigames.com": {"CAT_001"},

	// Adult networks / traffic
	"exoclick.com":     {"CAT_001"},
	"juicyads.com":     {"CAT_001"},
	"trafficjunky.com": {"CAT_001"},
	"pornrevenue.com":  {"CAT_001"},
	"crakrevenue.com":  {"CAT_001"},

	// ── CAT_005: Gaming ──

	// Major platforms
	"steamcommunity.com": {"CAT_005"},
	"steampowered.com":   {"CAT_005"},
	"epicgames.com":      {"CAT_005"},
	"battle.net":         {"CAT_005"},
	"blizzard.com":       {"CAT_005"},
	"playstation.com":    {"CAT_005"},
	"xbox.com":           {"CAT_005"},
	"nintendo.com":       {"CAT_005"},
	"origin.com":         {"CAT_005"},
	"ea.com":             {"CAT_005"},
	"ubisoft.com":        {"CAT_005"},
	"gog.com":            {"CAT_005"},
	"humblebundle.com":   {"CAT_005"},
	"itch.io":            {"CAT_005"},
	"gamejolt.com":       {"CAT_005"},
	"newgrounds.com":     {"CAT_005"},

	// Game publishers / studios
	"activision.com":      {"CAT_005"},
	"riotgames.com":       {"CAT_005"},
	"leagueoflegends.com": {"CAT_005"},
	"valorant.com":        {"CAT_005"},
	"valvesoftware.com":   {"CAT_005"},
	"mojang.com":          {"CAT_005"},
	"minecraft.net":       {"CAT_005"},
	"bethesda.net":        {"CAT_005"},
	"capcom.com":          {"CAT_005"},
	"konami.com":          {"CAT_005"},
	"sega.com":            {"CAT_005"},
	"bandainamco.com":     {"CAT_005"},
	"rockstargames.com":   {"CAT_005"},
	"2k.com":              {"CAT_005"},
	"take2games.com":      {"CAT_005"},
	"paradoxplaza.com":    {"CAT_005"},
	"paradoxinteractive.com": {"CAT_005"},
	"deepsilver.com":      {"CAT_005"},
	"cdprojektred.com":    {"CAT_005"},
	"innersloth.com":      {"CAT_005"},
	"supercell.com":       {"CAT_005"},
	"king.com":            {"CAT_005"},
	"gameloft.com":        {"CAT_005"},

	// First-party game domains
	"overwatch.com":       {"CAT_005"},
	"worldofwarcraft.com": {"CAT_005"},
	"hearthstone.com":     {"CAT_005"},
	"callofduty.com":      {"CAT_005"},
	"dota2.com":           {"CAT_005"},
	"fortnite.com":        {"CAT_005"},
	"rocketleague.com":    {"CAT_005"},
	"eldenring.com":       {"CAT_005"},
	"cyberpunk.net":       {"CAT_005"},

	// Game media
	"ign.com":           {"CAT_005"},
	"gamespot.com":      {"CAT_005"},
	"eurogamer.net":     {"CAT_005"},
	"polygon.com":       {"CAT_005"},
	"kotaku.com":        {"CAT_005"},
	"gamefaqs.com":      {"CAT_005"},
	"pcgamer.com":       {"CAT_005"},
	"vg247.com":         {"CAT_005"},

	// Esports / streaming
	"twitch.tv":         {"CAT_005"},
	"trovo.live":        {"CAT_005"},

	// Game tools
	"gamepedia.com":     {"CAT_005"},
	"curseforge.com":    {"CAT_005"},
	"fandom.com":        {"CAT_005"},

	// Chinese gaming
	"netease.com":       {"CAT_005"},
	//"163.com":           {"CAT_005"}, // removed — portal/email, not gaming
	"mihoyo.com":        {"CAT_005"},
	"hoyoverse.com":     {"CAT_005"},
	"genshin-impact.com": {"CAT_005"},
	"bilibili.com":      {"CAT_005"},
	"blizzard.cn":       {"CAT_005"},
	"wegame.com":        {"CAT_005"},
	"taptap.io":         {"CAT_005"},
	"taptap.com":        {"CAT_005"},

	// Roblox
	"roblox.com":        {"CAT_005"},
}
// ── JNI Helper Functions ──

func jstringToString(env *C.JNIEnv, js C.jstring) string {
	if js == 0 {
		return ""
	}
	chars := C.call_GetStringUTFChars(env, js)
	if chars == nil {
		return ""
	}
	defer C.call_ReleaseStringUTFChars(env, js, chars)
	return C.GoString(chars)
}

func stringToJstring(env *C.JNIEnv, s string) C.jstring {
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	return C.call_NewStringUTF(env, cs)
}

// ── Initialization ──

//export Java_com_dnsway_app_engine_DnsEngine_nativeInit
func Java_com_dnsway_app_engine_DnsEngine_nativeInit(env *C.JNIEnv, cls C.jclass, dataDir C.jstring) C.jboolean {
	dataPath := jstringToString(env, dataDir)
	log.Printf("[DnsEngine] nativeInit with dataDir: %s", dataPath)

	// Built-in domain-category mappings (used when no CSV is loaded)
	mu.Lock()
	for d, cats := range builtinCategoryDB {
		existing := categoryDB[d]
		existing = append(existing, cats...)
		categoryDB[d] = existing
	}
	mu.Unlock()

	_ = dataPath
	atomic.StoreInt32(&initialized, 1)
	return C.JNI_TRUE
}

//export Java_com_dnsway_app_engine_DnsEngine_nativeLoadCategories
func Java_com_dnsway_app_engine_DnsEngine_nativeLoadCategories(env *C.JNIEnv, cls C.jclass, path C.jstring) C.jint {
	csvPath := jstringToString(env, path)
	f, err := os.Open(csvPath)
	if err != nil {
		log.Printf("[DnsEngine] failed to open categories CSV: %v", err)
		return 0
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		log.Printf("[DnsEngine] failed to read categories CSV: %v", err)
		return 0
	}

	mu.Lock()
	defer mu.Unlock()

	count := 0
	seen := make(map[string]bool) // dedup "domain:category"
	for _, rec := range records {
		if len(rec) < 2 {
			continue
		}
		domain := strings.TrimSpace(strings.ToLower(rec[0]))
		catID := strings.TrimSpace(strings.ToUpper(rec[1]))
		if domain == "" || catID == "" {
			continue
		}
		key := domain + ":" + catID
		if seen[key] {
			continue
		}
		seen[key] = true
		categoryDB[domain] = append(categoryDB[domain], catID)
		count++
	}

	log.Printf("[DnsEngine] loaded %d domain→category mappings from CSV", count)
	return C.int(count)
}

// ── Domain Processing ──

//export Java_com_dnsway_app_engine_DnsEngine_nativeProcessDomain
func Java_com_dnsway_app_engine_DnsEngine_nativeProcessDomain(env *C.JNIEnv, cls C.jclass, domain C.jstring) C.jstring {
	d := jstringToString(env, domain)
	if d == "" {
		return stringToJstring(env, "ALLOW")
	}

	d = strings.TrimSpace(strings.ToLower(d))
	atomic.AddInt32(&totalQueries, 1)

	mu.RLock()
	defer mu.RUnlock()

	// P0: Bypass domains (VPN/proxy/public DNS)
	if isBypassDomain(d) {
		atomic.AddInt32(&blockedCount, 1)
		return stringToJstring(env, "BLOCK:P0 Bypass Domain")
	}

	// P1: Allowlist
	if isInAllowlist(d) {
		return stringToJstring(env, "ALLOW")
	}

	// P2: Denylist
	if isInDenylist(d) {
		atomic.AddInt32(&blockedCount, 1)
		return stringToJstring(env, "BLOCK:P2 Denylist")
	}

	// P3: Category filtering
	if !isForcedAllowed(d) {
		cats := lookupCategories(d)
		if isAnyCategoryBlocked(cats) {
			atomic.AddInt32(&blockedCount, 1)
			return stringToJstring(env, "BLOCK:P3 Category:"+strings.Join(cats, ","))
		}
	}

	// P5.5: SafeSearch rewrite
	if safeSearchEnabled {
		if rewrite := getSafeSearchRewrite(d); rewrite != "" {
			return stringToJstring(env, "ALLOW") // SafeSearch handled at DNS level
		}
	}

	// P6: Default allow
	return stringToJstring(env, "ALLOW")
}

// ── Allowlist / Denylist ──

//export Java_com_dnsway_app_engine_DnsEngine_nativeAddAllowlist
func Java_com_dnsway_app_engine_DnsEngine_nativeAddAllowlist(env *C.JNIEnv, cls C.jclass, domain C.jstring) C.jboolean {
	d := jstringToString(env, domain)
	if d == "" {
		return C.JNI_FALSE
	}
	d = strings.TrimSpace(strings.ToLower(d))
	mu.Lock()
	allowlist[d] = true
	mu.Unlock()
	return C.JNI_TRUE
}

//export Java_com_dnsway_app_engine_DnsEngine_nativeRemoveAllowlist
func Java_com_dnsway_app_engine_DnsEngine_nativeRemoveAllowlist(env *C.JNIEnv, cls C.jclass, domain C.jstring) C.jboolean {
	d := jstringToString(env, domain)
	if d == "" {
		return C.JNI_FALSE
	}
	d = strings.TrimSpace(strings.ToLower(d))
	mu.Lock()
	delete(allowlist, d)
	mu.Unlock()
	return C.JNI_TRUE
}

//export Java_com_dnsway_app_engine_DnsEngine_nativeAddDenylist
func Java_com_dnsway_app_engine_DnsEngine_nativeAddDenylist(env *C.JNIEnv, cls C.jclass, domain C.jstring, reason C.jstring) C.jboolean {
	d := jstringToString(env, domain)
	if d == "" {
		return C.JNI_FALSE
	}
	d = strings.TrimSpace(strings.ToLower(d))
	mu.Lock()
	denylist[d] = true
	mu.Unlock()
	return C.JNI_TRUE
}

//export Java_com_dnsway_app_engine_DnsEngine_nativeRemoveDenylist
func Java_com_dnsway_app_engine_DnsEngine_nativeRemoveDenylist(env *C.JNIEnv, cls C.jclass, domain C.jstring) C.jboolean {
	d := jstringToString(env, domain)
	if d == "" {
		return C.JNI_FALSE
	}
	d = strings.TrimSpace(strings.ToLower(d))
	mu.Lock()
	delete(denylist, d)
	mu.Unlock()
	return C.JNI_TRUE
}

// ── Category Toggles ──

//export Java_com_dnsway_app_engine_DnsEngine_nativeSetCategory
func Java_com_dnsway_app_engine_DnsEngine_nativeSetCategory(env *C.JNIEnv, cls C.jclass, categoryID C.jstring, enabled C.jboolean) C.jboolean {
	catID := jstringToString(env, categoryID)
	if catID == "" {
		return C.JNI_FALSE
	}
	mu.Lock()
	categoryConfig[catID] = enabled == C.JNI_TRUE
	mu.Unlock()
	return C.JNI_TRUE
}

// ── Stats ──

//export Java_com_dnsway_app_engine_DnsEngine_nativeGetStats
func Java_com_dnsway_app_engine_DnsEngine_nativeGetStats(env *C.JNIEnv, cls C.jclass) C.jstring {
	stats := map[string]int{
		"total":   int(atomic.LoadInt32(&totalQueries)),
		"blocked": int(atomic.LoadInt32(&blockedCount)),
	}
	b, _ := json.Marshal(stats)
	return stringToJstring(env, string(b))
}

// ── SafeSearch ──

//export Java_com_dnsway_app_engine_DnsEngine_nativeShouldSafeSearch
func Java_com_dnsway_app_engine_DnsEngine_nativeShouldSafeSearch(env *C.JNIEnv, cls C.jclass, domain C.jstring) C.jboolean {
	if !safeSearchEnabled {
		return C.JNI_FALSE
	}
	d := jstringToString(env, domain)
	if d == "" {
		return C.JNI_FALSE
	}
	d = strings.TrimSpace(strings.ToLower(d))
	mu.RLock()
	defer mu.RUnlock()
	return boolToJBool(shouldSafeSearchDomain(d))
}

//export Java_com_dnsway_app_engine_DnsEngine_nativeSetSafeSearch
func Java_com_dnsway_app_engine_DnsEngine_nativeSetSafeSearch(env *C.JNIEnv, cls C.jclass, enabled C.jboolean) {
	safeSearchEnabled = enabled == C.JNI_TRUE
}

// ── Decision Logic (copied from engine.go, simplified for standalone use) ──

var bypassDomains = map[string]bool{
	"nordvpn.com": true, "expressvpn.com": true, "surfshark.com": true,
	"protonvpn.com": true, "windscribe.com": true, "hotspotshield.com": true,
	"tunnelbear.com": true, "vpnunlimited.com": true, "ipvanish.com": true,
	"hide.me": true, "kproxy.com": true, "proxysite.com": true,
	"torproject.org": true, "v2ray.com": true, "v2fly.org": true, "shadowsocks.org": true,
	"dns.google": true, "dns.google.com": true,
	"cloudflare-dns.com": true, "one.one.one.one": true, "1.1.1.1": true, "1.0.0.1": true,
	"dns.quad9.net": true, "9.9.9.9": true,
	"doh.opendns.com": true, "dns.opendns.com": true,
	"dns.adguard.com": true, "doh.adguard.com": true,
	"doh.cleanbrowsing.org": true,
	"dns.nextdns.io": true,
	"dns.alidns.com": true, "doh.pub": true, "dns.qq.com": true,
	"httpdns.qq.com": true, "httpdns.tencent.com": true,
	"connectivitycheck.gstatic.com": true,
	"connectivitycheck.android.com": true,
	"clients3.google.com": true,
}

var forcedAllow = map[string]bool{
	"google.com": true, "googleapis.com": true, "gstatic.com": true,
	"googlevideo.com": true, "youtube.com": true, "ytimg.com": true,
	"googleusercontent.com": true,
	"microsoft.com": true, "azure.com": true, "office.com": true,
	"apple.com": true, "icloud.com": true,
	"cloudflare.com": true, "cloudflareinsights.com": true,
	"amazon.com": true, "amazonaws.com": true, "cloudfront.net": true,
	"facebook.com": true, "fbcdn.net": true, "instagram.com": true,
	"whatsapp.com": true, "messenger.com": true,
	"jsdelivr.net": true, "unpkg.com": true, "cdnjs.cloudflare.com": true,
	"fonts.googleapis.com": true, "fonts.gstatic.com": true,
	"github.com": true, "githubusercontent.com": true,
	"gitlab.com": true, "docker.com": true, "docker.io": true,
	"openai.com": true, "chatgpt.com": true, "oaistatic.com": true,
	"twitter.com": true, "x.com": true, "twimg.com": true,
	"linkedin.com": true, "discord.com": true, "discordapp.net": true,
	"telegram.org": true, "t.me": true,
	"weixin.qq.com": true, "qq.com": true,
	"taobao.com": true, "tmall.com": true, "jd.com": true,
	"paypal.com": true, "stripe.com": true,
	"adobe.com": true, "adobedtm.com": true,
	"android.clients.google.com": true,

	// Anti-false-positive overrides — these are NOT adult despite CSV saying so
	"baidu.com": true, "baidustatic.com": true, "bing.com": true,
	"akamai.net": true, "ebay.com": true, "imgur.com": true,
	"soundcloud.com": true, "csdn.net": true, "xiaohongshu.com": true,
	"163.com": true, // portal/email, not gaming
	"douyin.com": true, "tencent.com": true,
	"vivo.com": true, "xiaomi.com": true,
}

var safeSearchRewrites = map[string]string{
	"google.com":         "forcesafesearch.google.com",
	"www.google.com":     "forcesafesearch.google.com",
	"images.google.com":  "forcesafesearch.google.com",
	"bing.com":           "strict.bing.com",
	"www.bing.com":       "strict.bing.com",
	"duckduckgo.com":     "safe.duckduckgo.com",
	"www.duckduckgo.com": "safe.duckduckgo.com",
}

func isBypassDomain(domain string) bool {
	for d := range bypassDomains {
		if domain == d || strings.HasSuffix(domain, "."+d) {
			return true
		}
	}
	return false
}

func isInAllowlist(domain string) bool {
	for d := range allowlist {
		if domain == d || strings.HasSuffix(domain, "."+d) {
			return true
		}
	}
	return false
}

func isInDenylist(domain string) bool {
	for d := range denylist {
		if domain == d || strings.HasSuffix(domain, "."+d) {
			return true
		}
	}
	return false
}

func lookupCategories(domain string) []string {
	mu.RLock()
	defer mu.RUnlock()
	if cats, ok := categoryDB[domain]; ok {
		return cats
	}
	for {
		dot := strings.IndexByte(domain, '.')
		if dot < 0 {
			return nil
		}
		domain = domain[dot+1:]
		if cats, ok := categoryDB[domain]; ok {
			return cats
		}
	}
}

func isForcedAllowed(domain string) bool {
	for d := range forcedAllow {
		if domain == d || strings.HasSuffix(domain, "."+d) {
			return true
		}
	}
	return false
}

func isAnyCategoryBlocked(cats []string) bool {
	for _, cat := range cats {
		if blocked, ok := categoryConfig[cat]; ok && blocked {
			return true
		}
	}
	return false
}

func shouldSafeSearchDomain(domain string) bool {
	_, ok := safeSearchRewrites[domain]
	return ok
}

func getSafeSearchRewrite(domain string) string {
	return safeSearchRewrites[domain]
}

func boolToJBool(b bool) C.jboolean {
	if b {
		return C.JNI_TRUE
	}
	return C.JNI_FALSE
}

// ── Main function (required for c-shared library) ──

func main() {}

// ── CSV Export for bundled data ──

// ExportDomainCategories writes the current category database to a CSV file.
// Called from the Android app's category data extraction code.
func ExportDomainCategories(path string) error {
	mu.RLock()
	defer mu.RUnlock()

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for domain, cats := range categoryDB {
		for _, cat := range cats {
			_, _ = w.WriteString(domain + "," + cat + "\n")
		}
	}
	return w.Flush()
}

// ── Entropy calculation for AI threat detection ──

func shannonEntropy(s string) float64 {
	freq := map[byte]int{}
	for i := 0; i < len(s); i++ {
		freq[s[i]]++
	}
	var entropy float64
	n := float64(len(s))
	for _, count := range freq {
		p := float64(count) / n
		entropy -= p * math.Log2(p)
	}
	return entropy
}
