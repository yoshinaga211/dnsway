package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"dnsway-pc/internal/auth"
	"dnsway-pc/internal/engine"
	"dnsway-pc/internal/repository"

	"github.com/gin-gonic/gin"
)

// ================================================================
// Data Flow Integration Test — 验证每个前端操作是否真实影响 DNS 引擎决策
// ================================================================

// setupDataFlowTest 创建完整的服务器路由 + mock 仓库，返回已认证的 token
func setupDataFlowTest(t *testing.T) (*APIHandler, *gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	repo := repository.NewTestRepository(dir)
	eng := engine.NewEngine()
	handler := NewAPIHandler(repo, eng)

	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/register", handler.Register)
		v1.POST("/auth/login", handler.Login)
		v1.POST("/auth/wechat/qrcode", handler.WechatQRCode)
		v1.GET("/auth/wechat/callback", handler.WechatCallback)
		v1.GET("/plans", handler.GetPlans)

		v1.POST("/payment/create-order", auth.AuthMiddleware(), handler.CreatePaymentOrder)
		v1.POST("/payment/wechat/notify", handler.WechatPaymentNotify)
		v1.GET("/payment/mock-pay", handler.MockPayConfirm)

		v1.POST("/stripe/create-checkout-session", auth.AuthMiddleware(), handler.CreateStripeCheckoutSession)
		v1.POST("/stripe/webhook", handler.StripeWebhook)
		v1.GET("/stripe/plans", handler.GetStripePlans)
		v1.GET("/stripe/subscription-status", auth.AuthMiddleware(), handler.GetStripeSubscriptionStatus)
		v1.POST("/stripe/cancel-subscription", auth.AuthMiddleware(), handler.CancelStripeSubscription)
		v1.GET("/stripe/mock-success", handler.StripeMockSuccess)

		userGroup := v1.Group("/user")
		userGroup.Use(auth.AuthMiddleware())
		{
			userGroup.GET("/profile", handler.GetUserProfile)
			userGroup.GET("/subscription", handler.GetUserSubscription)
		}

		profiles := v1.Group("/profiles")
		profiles.Use(auth.AuthMiddleware())
		{
			profiles.GET("", handler.ListProfiles)
			p := profiles.Group("/:profileId")
			{
				p.GET("/parental-control", handler.GetParentalControl)
				p.PATCH("/parental-control/categories", handler.UpdateCategories)
				p.PATCH("/settings", handler.UpdateSettings)
				p.GET("/allowlist", handler.GetAllowlist)
				p.POST("/allowlist", handler.UpdateAllowlist)
				p.DELETE("/allowlist/:domain", handler.DeleteAllowlist)
				p.GET("/denylist", handler.GetDenylist)
				p.POST("/denylist", handler.UpdateDenylist)
				p.DELETE("/denylist/:domain", handler.DeleteDenylist)
				p.GET("/time-windows", handler.GetTimeWindows)
				p.POST("/time-windows", handler.AddTimeWindow)
				p.DELETE("/time-windows/:id", handler.DeleteTimeWindow)
				p.GET("/history", handler.GetHistory)
				p.GET("/stats", handler.GetStats)
				p.GET("/query", handler.SimulateQuery)
			}
		}
	}

	// 注册用户获取 token
	w := request(r, "POST", "/api/v1/auth/register", `{"email":"dataflow@test.com","password":"testpass123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("register failed: %d %s", w.Code, w.Body.String())
	}
	var regResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &regResp)
	token, _ := regResp["token"].(string)
	if token == "" {
		t.Fatal("empty token")
	}

	return handler, r, token
}

// simulateQuery 调用 query API 并返回 decision + reason
func simulateQuery(r *gin.Engine, token, profileID, domain string) (int, string) {
	w := authRequest(r, "GET",
		fmt.Sprintf("/api/v1/profiles/%s/query?domain=%s", profileID, domain),
		"", token)
	if w.Code != http.StatusOK {
		return -1, fmt.Sprintf("HTTP %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	decision := int(resp["decision"].(float64))
	reason, _ := resp["reason"].(string)
	return decision, reason
}

// assertDecision 辅助：验证 query 结果是否符合预期
func assertDecision(t *testing.T, token, profileID, domain string, r *gin.Engine, expectedDecision int, expectedReasonPrefix string) {
	t.Helper()
	d, reason := simulateQuery(r, token, profileID, domain)
	if d != expectedDecision {
		t.Errorf("[%s] expected decision=%d, got %d (reason: %s)", domain, expectedDecision, d, reason)
	}
	if expectedReasonPrefix != "" && !stringsHasPrefix(reason, expectedReasonPrefix) {
		t.Errorf("[%s] expected reason prefix %q, got %q", domain, expectedReasonPrefix, reason)
	}
}

func stringsHasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

// ================================================================
// 测试 1: Parental Control Tab — 分类开关真实改变拦截结果
// ================================================================

func TestDataFlow_CategoryToggle_BlocksAndUnblocks(t *testing.T) {
	_, r, token := setupDataFlowTest(t)
	pid := "test"

	// 初始: CAT_001 拦截, CAT_002 (Gambling) 放行 (无时间窗口)
	assertDecision(t, token, pid, "pornhub.com", r, 1, "P3: Content Category Blocked")
	assertDecision(t, token, pid, "badoo.com", r, 0, "")

	// 开启 CAT_003 (Dating)
	resp := authRequest(r, "PATCH",
		"/api/v1/profiles/test/parental-control/categories",
		`{"CAT_003":true}`, token)
	if resp.Code != 200 {
		t.Fatalf("toggle CAT_003 failed: %s", resp.Body.String())
	}
	assertDecision(t, token, pid, "badoo.com", r, 1, "P3: Content Category Blocked")

	// 关闭 CAT_001 (色情)
	resp = authRequest(r, "PATCH",
		"/api/v1/profiles/test/parental-control/categories",
		`{"CAT_001":false}`, token)
	if resp.Code != 200 {
		t.Fatalf("toggle CAT_001 off failed: %s", resp.Body.String())
	}
	assertDecision(t, token, pid, "pornhub.com", r, 0, "")
}

// ================================================================
// 测试 2: Allowlist Tab — 添加域名后绕过分类拦截
// ================================================================

func TestDataFlow_Allowlist_BypassesCategoryBlock(t *testing.T) {
	_, r, token := setupDataFlowTest(t)
	pid := "test"

	assertDecision(t, token, pid, "pornhub.com", r, 1, "")

	resp := authRequest(r, "POST",
		"/api/v1/profiles/test/allowlist",
		`{"domain":"pornhub.com","allowed":true}`, token)
	if resp.Code != 200 {
		t.Fatalf("add allowlist failed: %s", resp.Body.String())
	}
	assertDecision(t, token, pid, "pornhub.com", r, 0, "P1: User Allowlist")

	resp = authRequest(r, "DELETE",
		"/api/v1/profiles/test/allowlist/pornhub.com", "", token)
	if resp.Code != 200 {
		t.Fatalf("remove allowlist failed: %s", resp.Body.String())
	}
	assertDecision(t, token, pid, "pornhub.com", r, 1, "P3: Content Category Blocked")
}

// ================================================================
// 测试 3: Denylist Tab — 添加域名后始终拦截
// ================================================================

func TestDataFlow_Denylist_AlwaysBlocks(t *testing.T) {
	_, r, token := setupDataFlowTest(t)
	pid := "test"

	assertDecision(t, token, pid, "google.com", r, 0, "")

	resp := authRequest(r, "POST",
		"/api/v1/profiles/test/denylist",
		`{"domain":"google.com","reason":"test block"}`, token)
	if resp.Code != 200 {
		t.Fatalf("add denylist failed: %s", resp.Body.String())
	}
	assertDecision(t, token, pid, "google.com", r, 1, "P2: User Denylist")

	resp = authRequest(r, "DELETE",
		"/api/v1/profiles/test/denylist/google.com", "", token)
	if resp.Code != 200 {
		t.Fatalf("remove denylist failed: %s", resp.Body.String())
	}
	assertDecision(t, token, pid, "google.com", r, 0, "")
}

// ================================================================
// 测试 4: Security Tab — 每个安全功能逐一验证拦截效果
// ================================================================

func TestDataFlow_Security_PhishingProtection(t *testing.T) {
	_, r, token := setupDataFlowTest(t)
	pid := "test"

	assertDecision(t, token, pid, "paypa1.com", r, 0, "")
	resp := authRequest(r, "PATCH", "/api/v1/profiles/test/settings",
		`{"phishing_protection":true}`, token)
	if resp.Code != 200 {
		t.Fatalf("enable phishing failed: %s", resp.Body.String())
	}
	assertDecision(t, token, pid, "paypa1.com", r, 1, "P0.5: Phishing Protection")
	assertDecision(t, token, pid, "xn--paypl-8na.com", r, 1, "P0.5: Phishing Protection")

	resp = authRequest(r, "PATCH", "/api/v1/profiles/test/settings",
		`{"phishing_protection":false}`, token)
	if resp.Code != 200 {
		t.Fatalf("disable phishing failed: %s", resp.Body.String())
	}
	assertDecision(t, token, pid, "paypa1.com", r, 0, "")
}

func TestDataFlow_Security_CryptoJackingProtection(t *testing.T) {
	_, r, token := setupDataFlowTest(t)
	pid := "test"

	assertDecision(t, token, pid, "coinhive.com", r, 0, "")
	resp := authRequest(r, "PATCH", "/api/v1/profiles/test/settings",
		`{"crypto_jacking_protection":true}`, token)
	if resp.Code != 200 {
		t.Fatalf("enable crypto failed: %s", resp.Body.String())
	}
	assertDecision(t, token, pid, "coinhive.com", r, 1, "P0.6: Crypto Mining Protection")
	assertDecision(t, token, pid, "crypto-loot.com", r, 1, "P0.6: Crypto Mining Protection")

	resp = authRequest(r, "PATCH", "/api/v1/profiles/test/settings",
		`{"crypto_jacking_protection":false}`, token)
	if resp.Code != 200 {
		t.Fatalf("disable crypto failed: %s", resp.Body.String())
	}
}

func TestDataFlow_Security_SuspectTLDBlocking(t *testing.T) {
	_, r, token := setupDataFlowTest(t)
	pid := "test"

	assertDecision(t, token, pid, "free-money.racing", r, 0, "")
	resp := authRequest(r, "PATCH", "/api/v1/profiles/test/settings",
		`{"suspect_tld_blocking":true}`, token)
	if resp.Code != 200 {
		t.Fatalf("enable TLD blocking failed: %s", resp.Body.String())
	}
	assertDecision(t, token, pid, "free-money.racing", r, 1, "P0.7: Suspect TLD Blocking")
	assertDecision(t, token, pid, "malware.tk", r, 1, "P0.7: Suspect TLD Blocking")
	assertDecision(t, token, pid, "example.com", r, 0, "")
}

func TestDataFlow_Security_AIThreatDetection(t *testing.T) {
	_, r, token := setupDataFlowTest(t)
	pid := "test"

	assertDecision(t, token, pid, "xqkzmnvbpqhrdtwlgjsf.com", r, 0, "")
	resp := authRequest(r, "PATCH", "/api/v1/profiles/test/settings",
		`{"ai_threat_detection":true}`, token)
	if resp.Code != 200 {
		t.Fatalf("enable AI detection failed: %s", resp.Body.String())
	}
	assertDecision(t, token, pid, "xqkzmnvbpqhrdtwlgjsf.com", r, 1, "P5: AI Threat Detection")
	assertDecision(t, token, pid,
		"abcdefghijklmnopqrstuvwxyz1234567890abcdef.com", r, 1, "P5: AI Threat Detection")
}

func TestDataFlow_Security_BypassPrevention(t *testing.T) {
	_, r, token := setupDataFlowTest(t)
	pid := "test"

	assertDecision(t, token, pid, "nordvpn.com", r, 0, "")
	assertDecision(t, token, pid, "dns.google", r, 0, "")
	resp := authRequest(r, "PATCH", "/api/v1/profiles/test/settings",
		`{"bypass_prevention":true}`, token)
	if resp.Code != 200 {
		t.Fatalf("enable bypass prevention failed: %s", resp.Body.String())
	}
	assertDecision(t, token, pid, "nordvpn.com", r, 1, "P0: Bypass Prevention")
	assertDecision(t, token, pid, "dns.google", r, 1, "P0: Bypass Prevention")
	assertDecision(t, token, pid, "cloudflare-dns.com", r, 1, "P0: Bypass Prevention")
}

// ================================================================
// 测试 5: SafeSearch — 搜索引擎域名被正确重写
// ================================================================

func TestDataFlow_SafeSearch_RewritesDomain(t *testing.T) {
	_, r, token := setupDataFlowTest(t)

	// 默认 SafeSearch=true → google.com 被重写
	resp := authRequest(r, "GET",
		"/api/v1/profiles/test/query?domain=google.com", "", token)
	if resp.Code != 200 {
		t.Fatalf("query failed: %s", resp.Body.String())
	}
	var result map[string]interface{}
	json.Unmarshal(resp.Body.Bytes(), &result)

	if result["resolved_domain"] != "forcesafesearch.google.com" {
		t.Errorf("expected SafeSearch rewrite to forcesafesearch.google.com, got %v",
			result["resolved_domain"])
	}
	reason, _ := result["reason"].(string)
	if !stringsHasPrefix(reason, "P5.5: SafeSearch Rewrite") {
		t.Errorf("expected SafeSearch rewrite reason, got %s", reason)
	}

	// 关闭 SafeSearch → google.com 不再被重写
	resp = authRequest(r, "PATCH", "/api/v1/profiles/test/settings",
		`{"safe_search":false}`, token)
	if resp.Code != 200 {
		t.Fatalf("disable safesearch failed: %s", resp.Body.String())
	}

	resp = authRequest(r, "GET",
		"/api/v1/profiles/test/query?domain=google.com", "", token)
	json.Unmarshal(resp.Body.Bytes(), &result)
	if result["resolved_domain"] == "forcesafesearch.google.com" {
		t.Errorf("SafeSearch should be disabled, domain was rewritten")
	}
}

// ================================================================
// 测试 6: 设置持久化 — 写入后读取验证
// ================================================================

func TestDataFlow_Settings_PersistAcrossReads(t *testing.T) {
	_, r, token := setupDataFlowTest(t)

	updates := []string{
		`{"safe_search":false}`,
		`{"youtube_restricted":false}`,
		`{"log_retention_days":30}`,
		`{"phishing_protection":true}`,
		`{"ai_threat_detection":true}`,
		`{"crypto_jacking_protection":true}`,
		`{"suspect_tld_blocking":true}`,
		`{"bypass_prevention":true}`,
		`{"storage_region":"eu"}`,
	}

	for _, body := range updates {
		resp := authRequest(r, "PATCH",
			"/api/v1/profiles/test/settings", body, token)
		if resp.Code != 200 {
			t.Errorf("update setting %s failed: %s", body, resp.Body.String())
		}
	}

	resp := authRequest(r, "GET",
		"/api/v1/profiles/test/parental-control", "", token)
	if resp.Code != 200 {
		t.Fatalf("get config failed: %s", resp.Body.String())
	}

	var config map[string]interface{}
	json.Unmarshal(resp.Body.Bytes(), &config)

	tests := []struct {
		field string
		want  interface{}
	}{
		{"safe_search", false},
		{"youtube_restricted", false},
		{"log_retention_days", float64(30)},
		{"phishing_protection", true},
		{"ai_threat_detection", true},
		{"crypto_jacking_protection", true},
		{"suspect_tld_blocking", true},
		{"bypass_prevention", true},
		{"storage_region", "eu"},
	}

	for _, tc := range tests {
		got, ok := config[tc.field]
		if !ok {
			t.Errorf("field %q not found in config", tc.field)
			continue
		}
		if got != tc.want {
			t.Errorf("expected %s=%v, got %v", tc.field, tc.want, got)
		}
	}
}

// ================================================================
// 测试 7: 完整用户旅程 — 注册 → 配置 → DNS 过滤全流程
// ================================================================

func TestDataFlow_FullUserJourney(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	repo := repository.NewTestRepository(dir)
	eng := engine.NewEngine()
	handler := NewAPIHandler(repo, eng)

	r := gin.New()
	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/register", handler.Register)
		v1.POST("/auth/login", handler.Login)

		userGroup := v1.Group("/user")
		userGroup.Use(auth.AuthMiddleware())
		{
			userGroup.GET("/profile", handler.GetUserProfile)
			userGroup.GET("/subscription", handler.GetUserSubscription)
		}

		profiles := v1.Group("/profiles")
		profiles.Use(auth.AuthMiddleware())
		{
			profiles.GET("", handler.ListProfiles)
			p := profiles.Group("/:profileId")
			{
				p.GET("/parental-control", handler.GetParentalControl)
				p.PATCH("/parental-control/categories", handler.UpdateCategories)
				p.PATCH("/settings", handler.UpdateSettings)
				p.GET("/allowlist", handler.GetAllowlist)
				p.POST("/allowlist", handler.UpdateAllowlist)
				p.DELETE("/allowlist/:domain", handler.DeleteAllowlist)
				p.GET("/denylist", handler.GetDenylist)
				p.POST("/denylist", handler.UpdateDenylist)
				p.DELETE("/denylist/:domain", handler.DeleteDenylist)
				p.GET("/time-windows", handler.GetTimeWindows)
				p.POST("/time-windows", handler.AddTimeWindow)
				p.DELETE("/time-windows/:id", handler.DeleteTimeWindow)
				p.GET("/history", handler.GetHistory)
				p.GET("/stats", handler.GetStats)
				p.GET("/query", handler.SimulateQuery)
			}
		}
	}

	// Step 1: 注册新用户
	w := request(r, "POST", "/api/v1/auth/register",
		`{"email":"parent@family.com","password":"secure123"}`)
	if w.Code != 200 {
		t.Fatalf("register failed: %d %s", w.Code, w.Body.String())
	}
	var regResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &regResp)
	token := fmt.Sprint(regResp["token"])
	userObj := regResp["user"].(map[string]interface{})
	userID := fmt.Sprint(userObj["id"])

	// 验证试用订阅
	subObj := regResp["subscription"].(map[string]interface{})
	if fmt.Sprint(subObj["status"]) != "trialing" {
		t.Errorf("expected trial, got status=%v", subObj["status"])
	}

	// Step 2: 配置分类拦截
	w = authRequest(r, "PATCH",
		fmt.Sprintf("/api/v1/profiles/%s/parental-control/categories", userID),
		`{"CAT_001":true,"CAT_005":true,"CAT_006":true,"CAT_007":true}`, token)
	if w.Code != 200 {
		t.Fatalf("set categories failed: %s", w.Body.String())
	}

	// Step 3: 配置安全功能
	w = authRequest(r, "PATCH",
		fmt.Sprintf("/api/v1/profiles/%s/settings", userID),
		`{"phishing_protection":true,"crypto_jacking_protection":true,"suspect_tld_blocking":true,"ai_threat_detection":true,"bypass_prevention":true}`,
		token)
	if w.Code != 200 {
		t.Fatalf("set security failed: %s", w.Body.String())
	}

	t.Log("验证分类拦截:")
	assertDecision(t, token, userID, "pornhub.com", r, 1, "P3")
	assertDecision(t, token, userID, "steamcommunity.com", r, 1, "P3")
	assertDecision(t, token, userID, "youtube.com", r, 1, "P3")
	assertDecision(t, token, userID, "facebook.com", r, 1, "P3")

	t.Log("验证安全拦截:")
	assertDecision(t, token, userID, "paypa1.com", r, 1, "P0.5")
	assertDecision(t, token, userID, "coinhive.com", r, 1, "P0.6")
	assertDecision(t, token, userID, "free-money.tk", r, 1, "P0.7")
	assertDecision(t, token, userID, "xqkzmnvbpqhrdtwlgjsf.com", r, 1, "P5")
	assertDecision(t, token, userID, "nordvpn.com", r, 1, "P0")

	// Step 4: 白名单绕过
	w = authRequest(r, "POST",
		fmt.Sprintf("/api/v1/profiles/%s/allowlist", userID),
		`{"domain":"youtube.com","allowed":true}`, token)
	if w.Code != 200 {
		t.Fatalf("add allowlist failed: %s", w.Body.String())
	}
	assertDecision(t, token, userID, "youtube.com", r, 0, "P1: User Allowlist")

	// Step 5: 黑名单拦截
	w = authRequest(r, "POST",
		fmt.Sprintf("/api/v1/profiles/%s/denylist", userID),
		`{"domain":"example.com","reason":"test"}`, token)
	if w.Code != 200 {
		t.Fatalf("add denylist failed: %s", w.Body.String())
	}
	assertDecision(t, token, userID, "example.com", r, 1, "P2: User Denylist")

	// Step 6: 时间窗口
	w = authRequest(r, "POST",
		fmt.Sprintf("/api/v1/profiles/%s/time-windows", userID),
		`{"name":"Weekend Fun","days_of_week":[0,6],"start_time":"14:00","end_time":"18:00","timezone":"Asia/Shanghai","target_type":"CATEGORY","target_value":"CAT_005"}`, token)
	if w.Code != 200 {
		t.Fatalf("add time window failed: %s", w.Body.String())
	}

	w = authRequest(r, "GET",
		fmt.Sprintf("/api/v1/profiles/%s/time-windows", userID), "", token)
	if w.Code != 200 {
		t.Fatalf("get time windows failed: %s", w.Body.String())
	}
	var twResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &twResp)
	tws, _ := twResp["time_windows"].([]interface{})
	if len(tws) != 1 {
		t.Errorf("expected 1 time window, got %d", len(tws))
	}

	// Step 7: 查询历史 + 统计
	simulateQuery(r, token, userID, "pornhub.com")
	simulateQuery(r, token, userID, "steamcommunity.com")
	simulateQuery(r, token, userID, "pornhub.com")

	w = authRequest(r, "GET",
		fmt.Sprintf("/api/v1/profiles/%s/history", userID), "", token)
	if w.Code != 200 {
		t.Fatalf("get history failed: %s", w.Body.String())
	}
	var hist map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &hist)
	if hist["count"].(float64) < 3 {
		t.Errorf("expected >=3 history entries, got %v", hist["count"])
	}

	w = authRequest(r, "GET",
		fmt.Sprintf("/api/v1/profiles/%s/stats", userID), "", token)
	if w.Code != 200 {
		t.Fatalf("get stats failed: %s", w.Body.String())
	}
	var stats map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &stats)
	top, _ := stats["top_blocked"].(map[string]interface{})
	if top["pornhub.com"] == nil {
		t.Error("expected pornhub.com in top blocked stats")
	}

	t.Log("全流程测试通过 ✓")
}

// ================================================================
// 测试 8: 优先级 — 安全功能 > 白名单 (P0.x > P1)
// 引擎设计中安全功能优先级高于用户白名单，防止恶意域名绕过
// ================================================================

func TestDataFlow_Priority_SecurityOverAllowlist(t *testing.T) {
	_, r, token := setupDataFlowTest(t)
	pid := "test"

	resp := authRequest(r, "PATCH", "/api/v1/profiles/test/settings",
		`{"crypto_jacking_protection":true}`,
		token)
	if resp.Code != 200 {
		t.Fatalf("enable security failed: %s", resp.Body.String())
	}

	resp = authRequest(r, "POST", "/api/v1/profiles/test/allowlist",
		`{"domain":"coinhive.com","allowed":true}`, token)
	if resp.Code != 200 {
		t.Fatalf("add allowlist failed: %s", resp.Body.String())
	}

	// P0.6 (加密挖矿保护) > P1 (白名单): 安全功能优先
	assertDecision(t, token, pid, "coinhive.com", r, 1, "P0.6: Crypto Mining Protection")
}

// ================================================================
// 测试 9: 优先级 — 白名单 > 黑名单 (P1 > P2)
// 引擎中白名单先于黑名单检查，白名单中的域名不会被黑名单覆盖
// ================================================================

func TestDataFlow_Priority_AllowlistOverDenylist(t *testing.T) {
	_, r, token := setupDataFlowTest(t)
	pid := "test"

	resp := authRequest(r, "POST", "/api/v1/profiles/test/allowlist",
		`{"domain":"example.com","allowed":true}`, token)
	if resp.Code != 200 {
		t.Fatalf("add allowlist failed: %s", resp.Body.String())
	}

	resp = authRequest(r, "POST", "/api/v1/profiles/test/denylist",
		`{"domain":"example.com","reason":"test"}`, token)
	if resp.Code != 200 {
		t.Fatalf("add denylist failed: %s", resp.Body.String())
	}

	// P1 (白名单) > P2 (黑名单): 白名单优先
	assertDecision(t, token, pid, "example.com", r, 0, "P1: User Allowlist")
}

// ================================================================
// 测试 10: 开→关→开 安全功能，验证拦截状态可逆
// ================================================================

func TestDataFlow_Security_ToggleOnOff(t *testing.T) {
	_, r, token := setupDataFlowTest(t)
	pid := "test"

	// 开启
	resp := authRequest(r, "PATCH", "/api/v1/profiles/test/settings",
		`{"phishing_protection":true,"crypto_jacking_protection":true,"suspect_tld_blocking":true,"ai_threat_detection":true,"bypass_prevention":true}`,
		token)
	if resp.Code != 200 {
		t.Fatalf("enable failed: %s", resp.Body.String())
	}

	assertDecision(t, token, pid, "paypa1.com", r, 1, "")
	assertDecision(t, token, pid, "coinhive.com", r, 1, "")
	assertDecision(t, token, pid, "malware.tk", r, 1, "")
	assertDecision(t, token, pid, "nordvpn.com", r, 1, "")
	assertDecision(t, token, pid, "xqkzmnvbpqhrdtwlgjsf.com", r, 1, "")

	// 关闭
	resp = authRequest(r, "PATCH", "/api/v1/profiles/test/settings",
		`{"phishing_protection":false,"crypto_jacking_protection":false,"suspect_tld_blocking":false,"ai_threat_detection":false,"bypass_prevention":false}`,
		token)
	if resp.Code != 200 {
		t.Fatalf("disable failed: %s", resp.Body.String())
	}

	assertDecision(t, token, pid, "paypa1.com", r, 0, "")
	assertDecision(t, token, pid, "coinhive.com", r, 0, "")
	assertDecision(t, token, pid, "malware.tk", r, 0, "")
	assertDecision(t, token, pid, "nordvpn.com", r, 0, "")
	assertDecision(t, token, pid, "xqkzmnvbpqhrdtwlgjsf.com", r, 0, "")
}
