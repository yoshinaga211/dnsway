package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"dnsway-pc/internal/auth"
	"dnsway-pc/internal/engine"
	"dnsway-pc/internal/repository"

	"github.com/gin-gonic/gin"
)

// ================================================================
// Full Integration Setup — mirrors main.go route registration
// ================================================================

func setupIntegration(t *testing.T) (*APIHandler, *gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	repo := repository.NewTestRepository(dir)
	eng := engine.NewEngine()
	handler := NewAPIHandler(repo, eng)

	r := gin.New()

	// CORS (same as main.go)
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Static & templates (simulated — no actual template loading)
	r.GET("/login", func(c *gin.Context) {
		c.String(http.StatusOK, "login page mock")
	})
	r.GET("/pricing", func(c *gin.Context) {
		c.String(http.StatusOK, "pricing page mock")
	})
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "dashboard page mock")
	})
	r.GET("/dashboard", func(c *gin.Context) {
		c.String(http.StatusOK, "dashboard page mock")
	})
	r.GET("/payment/success", func(c *gin.Context) {
		c.String(http.StatusOK, "payment success page mock")
	})
	r.GET("/payment/cancel", func(c *gin.Context) {
		c.String(http.StatusOK, "payment cancel page mock")
	})

	// API v1 — exact routes from main.go
	v1 := r.Group("/api/v1")
	{
		// Public auth
		v1.POST("/auth/register", handler.Register)
		v1.POST("/auth/login", handler.Login)
		v1.POST("/auth/wechat/qrcode", handler.WechatQRCode)
		v1.GET("/auth/wechat/callback", handler.WechatCallback)

		// Public plans
		v1.GET("/plans", handler.GetPlans)

		// Payment routes
		v1.POST("/payment/create-order", auth.AuthMiddleware(), handler.CreatePaymentOrder)
		v1.POST("/payment/wechat/notify", handler.WechatPaymentNotify)
		v1.GET("/payment/mock-pay", handler.MockPayConfirm)

		// Stripe
		v1.POST("/stripe/create-checkout-session", auth.AuthMiddleware(), handler.CreateStripeCheckoutSession)
		v1.POST("/stripe/webhook", handler.StripeWebhook)
		v1.GET("/stripe/plans", handler.GetStripePlans)
		v1.GET("/stripe/subscription-status", auth.AuthMiddleware(), handler.GetStripeSubscriptionStatus)
		v1.POST("/stripe/cancel-subscription", auth.AuthMiddleware(), handler.CancelStripeSubscription)
		v1.GET("/stripe/mock-success", handler.StripeMockSuccess)

		// User (auth required)
		userGroup := v1.Group("/user")
		userGroup.Use(auth.AuthMiddleware())
		{
			userGroup.GET("/profile", handler.GetUserProfile)
			userGroup.GET("/subscription", handler.GetUserSubscription)
		}

		// Profiles (auth required)
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

	// Register a test user and get a token
	w := request(r, "POST", "/api/v1/auth/register", `{"email":"inttest@test.com","password":"testpass123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("register for integration test: %d %s", w.Code, w.Body.String())
	}
	var regResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &regResp)
	token, _ := regResp["token"].(string)
	if token == "" {
		t.Fatal("integration test: empty token after register")
	}

	return handler, r, token
}

func expectCode(t *testing.T, label string, actual, expected int, body string) {
	t.Helper()
	if actual != expected {
		t.Errorf("[%s] expected HTTP %d, got %d: %s", label, expected, actual, body)
	}
}

// ================================================================
// 1. Page Renders
// ================================================================

func TestIntegration_Pages(t *testing.T) {
	_, r, _ := setupIntegration(t)

	pages := []struct {
		path string
		code int
	}{
		{"/login", 200},
		{"/pricing", 200},
		{"/", 200},
		{"/dashboard", 200},
		{"/payment/success", 200},
		{"/payment/cancel", 200},
	}

	for _, p := range pages {
		w := request(r, "GET", p.path, "")
		expectCode(t, "GET "+p.path, w.Code, p.code, w.Body.String())
	}
}

// ================================================================
// 2. Auth Endpoints
// ================================================================

func TestIntegration_AuthRegister(t *testing.T) {
	_, r, _ := setupIntegration(t)

	// Success
	w := request(r, "POST", "/api/v1/auth/register", `{"email":"fresh@test.com","password":"pass1234"}`)
	expectCode(t, "register success", w.Code, 200, w.Body.String())

	// Duplicate
	w = request(r, "POST", "/api/v1/auth/register", `{"email":"fresh@test.com","password":"pass1234"}`)
	expectCode(t, "register duplicate", w.Code, 409, w.Body.String())

	// Missing email
	w = request(r, "POST", "/api/v1/auth/register", `{"password":"pass1234"}`)
	expectCode(t, "register no email", w.Code, 400, w.Body.String())

	// Missing password
	w = request(r, "POST", "/api/v1/auth/register", `{"email":"nopass@test.com"}`)
	expectCode(t, "register no password", w.Code, 400, w.Body.String())

	// Short password
	w = request(r, "POST", "/api/v1/auth/register", `{"email":"short@test.com","password":"12345"}`)
	expectCode(t, "register short pw", w.Code, 400, w.Body.String())

	// Invalid JSON
	w = request(r, "POST", "/api/v1/auth/register", `{bad-json}`)
	expectCode(t, "register bad json", w.Code, 400, w.Body.String())
}

func TestIntegration_AuthLogin(t *testing.T) {
	_, r, _ := setupIntegration(t)

	// Register first
	request(r, "POST", "/api/v1/auth/register", `{"email":"logintest@test.com","password":"pass1234"}`)

	// Success
	w := request(r, "POST", "/api/v1/auth/login", `{"email":"logintest@test.com","password":"pass1234"}`)
	expectCode(t, "login success", w.Code, 200, w.Body.String())

	// Wrong password
	w = request(r, "POST", "/api/v1/auth/login", `{"email":"logintest@test.com","password":"wrongpass"}`)
	expectCode(t, "login wrong pw", w.Code, 401, w.Body.String())

	// Non-existent user
	w = request(r, "POST", "/api/v1/auth/login", `{"email":"nobody@test.com","password":"pass1234"}`)
	expectCode(t, "login no user", w.Code, 401, w.Body.String())

	// Missing fields
	w = request(r, "POST", "/api/v1/auth/login", `{}`)
	expectCode(t, "login missing fields", w.Code, 400, w.Body.String())

	// Invalid JSON
	w = request(r, "POST", "/api/v1/auth/login", `not-json`)
	expectCode(t, "login bad json", w.Code, 400, w.Body.String())
}

func TestIntegration_AuthWechat(t *testing.T) {
	_, r, _ := setupIntegration(t)

	// QR Code
	w := request(r, "POST", "/api/v1/auth/wechat/qrcode", "")
	expectCode(t, "wechat qrcode", w.Code, 200, w.Body.String())

	// Callback without code
	w = request(r, "GET", "/api/v1/auth/wechat/callback", "")
	expectCode(t, "wechat callback no code", w.Code, 400, w.Body.String())
}

// ================================================================
// 3. Plans (public)
// ================================================================

func TestIntegration_Plans(t *testing.T) {
	_, r, _ := setupIntegration(t)

	w := request(r, "GET", "/api/v1/plans", "")
	expectCode(t, "get plans", w.Code, 200, w.Body.String())

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	plans, ok := resp["plans"].([]interface{})
	if !ok || len(plans) == 0 {
		t.Error("expected non-empty plans array")
	}
}

// ================================================================
// 4. User Endpoints (auth required)
// ================================================================

func TestIntegration_UserProfile(t *testing.T) {
	_, r, token := setupIntegration(t)

	// Authenticated
	w := authRequest(r, "GET", "/api/v1/user/profile", "", token)
	expectCode(t, "user profile authed", w.Code, 200, w.Body.String())

	// Unauthenticated
	w = request(r, "GET", "/api/v1/user/profile", "")
	expectCode(t, "user profile unauth", w.Code, 401, w.Body.String())

	// Invalid token
	w = authRequest(r, "GET", "/api/v1/user/profile", "", "invalid-token")
	expectCode(t, "user profile bad token", w.Code, 401, w.Body.String())
}

func TestIntegration_UserSubscription(t *testing.T) {
	_, r, token := setupIntegration(t)

	// Authenticated
	w := authRequest(r, "GET", "/api/v1/user/subscription", "", token)
	expectCode(t, "user sub authed", w.Code, 200, w.Body.String())

	// Unauthenticated
	w = request(r, "GET", "/api/v1/user/subscription", "")
	expectCode(t, "user sub unauth", w.Code, 401, w.Body.String())
}

// ================================================================
// 5. Profile endpoints (auth required)
// ================================================================

func TestIntegration_Profiles(t *testing.T) {
	_, r, token := setupIntegration(t)

	// List profiles (authed)
	w := authRequest(r, "GET", "/api/v1/profiles", "", token)
	expectCode(t, "list profiles authed", w.Code, 200, w.Body.String())

	// List profiles (unauth)
	w = request(r, "GET", "/api/v1/profiles", "")
	expectCode(t, "list profiles unauth", w.Code, 401, w.Body.String())
}

// ================================================================
// 6. Parental Control
// ================================================================

func TestIntegration_ParentalControl(t *testing.T) {
	_, r, token := setupIntegration(t)

	// Get config (existing profile)
	w := authRequest(r, "GET", "/api/v1/profiles/test/parental-control", "", token)
	expectCode(t, "get parental control", w.Code, 200, w.Body.String())

	// Toggle categories
	w = authRequest(r, "PATCH", "/api/v1/profiles/test/parental-control/categories",
		`{"CAT_001":true,"CAT_002":true}`, token)
	expectCode(t, "update categories", w.Code, 200, w.Body.String())

	// Invalid body
	w = authRequest(r, "PATCH", "/api/v1/profiles/test/parental-control/categories",
		`bad-json`, token)
	expectCode(t, "update categories bad body", w.Code, 400, w.Body.String())

	// Unauthenticated
	w = request(r, "GET", "/api/v1/profiles/test/parental-control", "")
	expectCode(t, "parental control unauth", w.Code, 401, w.Body.String())
}

// ================================================================
// 7. Settings
// ================================================================

func TestIntegration_Settings(t *testing.T) {
	_, r, token := setupIntegration(t)

	// Update settings
	w := authRequest(r, "PATCH", "/api/v1/profiles/test/settings",
		`{"safe_search":true,"phishing_protection":true,"log_retention_days":14}`, token)
	expectCode(t, "update settings", w.Code, 200, w.Body.String())

	// Verify
	w = authRequest(r, "GET", "/api/v1/profiles/test/parental-control", "", token)
	expectCode(t, "verify settings", w.Code, 200, w.Body.String())
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["log_retention_days"] != float64(14) {
		t.Errorf("expected log_retention_days=14, got %v", resp["log_retention_days"])
	}

	// Invalid body
	w = authRequest(r, "PATCH", "/api/v1/profiles/test/settings", `bad-json`, token)
	expectCode(t, "update settings bad body", w.Code, 400, w.Body.String())
}

// ================================================================
// 8. Allowlist CRUD
// ================================================================

func TestIntegration_Allowlist(t *testing.T) {
	_, r, token := setupIntegration(t)

	// Get (initial)
	w := authRequest(r, "GET", "/api/v1/profiles/test/allowlist", "", token)
	expectCode(t, "get allowlist", w.Code, 200, w.Body.String())

	// Add
	w = authRequest(r, "POST", "/api/v1/profiles/test/allowlist",
		`{"domain":"example.org","allowed":true}`, token)
	expectCode(t, "add allowlist", w.Code, 200, w.Body.String())

	// Verify added
	w = authRequest(r, "GET", "/api/v1/profiles/test/allowlist", "", token)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	list, _ := resp["allowlist"].([]interface{})
	found := false
	for _, d := range list {
		if d == "example.org" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected example.org in allowlist after add")
	}

	// Delete
	w = authRequest(r, "DELETE", "/api/v1/profiles/test/allowlist/example.org", "", token)
	expectCode(t, "delete allowlist", w.Code, 200, w.Body.String())

	// Invalid body
	w = authRequest(r, "POST", "/api/v1/profiles/test/allowlist", `bad-json`, token)
	expectCode(t, "add allowlist bad body", w.Code, 400, w.Body.String())

	// Unauthenticated
	w = request(r, "GET", "/api/v1/profiles/test/allowlist", "")
	expectCode(t, "get allowlist unauth", w.Code, 401, w.Body.String())

	// Unknown profile (should still 200)
	w = authRequest(r, "GET", "/api/v1/profiles/unknown/allowlist", "", token)
	expectCode(t, "get allowlist unknown profile", w.Code, 200, w.Body.String())
}

// ================================================================
// 9. Denylist CRUD
// ================================================================

func TestIntegration_Denylist(t *testing.T) {
	_, r, token := setupIntegration(t)

	// Get (initial)
	w := authRequest(r, "GET", "/api/v1/profiles/test/denylist", "", token)
	expectCode(t, "get denylist", w.Code, 200, w.Body.String())

	// Add
	w = authRequest(r, "POST", "/api/v1/profiles/test/denylist",
		`{"domain":"evil.example.com","reason":"testing"}`, token)
	expectCode(t, "add denylist", w.Code, 200, w.Body.String())

	// Verify added
	w = authRequest(r, "GET", "/api/v1/profiles/test/denylist", "", token)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	dlist, _ := resp["denylist"].([]interface{})
	found := false
	for _, entry := range dlist {
		e, _ := entry.(map[string]interface{})
		if e["domain"] == "evil.example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected evil.example.com in denylist after add")
	}

	// Delete
	w = authRequest(r, "DELETE", "/api/v1/profiles/test/denylist/evil.example.com", "", token)
	expectCode(t, "delete denylist", w.Code, 200, w.Body.String())

	// Invalid body
	w = authRequest(r, "POST", "/api/v1/profiles/test/denylist", `bad-json`, token)
	expectCode(t, "add denylist bad body", w.Code, 400, w.Body.String())

	// Unknown profile
	w = authRequest(r, "GET", "/api/v1/profiles/unknown/denylist", "", token)
	expectCode(t, "get denylist unknown profile", w.Code, 200, w.Body.String())
}

// ================================================================
// 10. Time Windows CRUD
// ================================================================

func TestIntegration_TimeWindows(t *testing.T) {
	_, r, token := setupIntegration(t)

	// Get (initial)
	w := authRequest(r, "GET", "/api/v1/profiles/test/time-windows", "", token)
	expectCode(t, "get time windows", w.Code, 200, w.Body.String())

	// Add
	w = authRequest(r, "POST", "/api/v1/profiles/test/time-windows",
		`{"name":"Test Window","days_of_week":[0,6],"start_time":"14:00","end_time":"18:00","timezone":"Asia/Shanghai","target_type":"CATEGORY","target_value":"CAT_005"}`, token)
	expectCode(t, "add time window", w.Code, 200, w.Body.String())

	// List and get the ID of the default window for deletion test
	w = authRequest(r, "GET", "/api/v1/profiles/test/time-windows", "", token)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	tws, _ := resp["time_windows"].([]interface{})
	if len(tws) > 0 {
		t.Logf("found %d time windows, testing delete", len(tws))
		tw1, _ := tws[0].(map[string]interface{})
		if id, ok := tw1["id"]; ok {
			idStr := fmt.Sprint(id)
			w = authRequest(r, "DELETE", "/api/v1/profiles/test/time-windows/"+idStr, "", token)
			expectCode(t, "delete time window", w.Code, 200, w.Body.String())
		}
	}

	// Invalid body
	w = authRequest(r, "POST", "/api/v1/profiles/test/time-windows", `bad-json`, token)
	expectCode(t, "add time window bad body", w.Code, 400, w.Body.String())

	// Unknown profile
	w = authRequest(r, "GET", "/api/v1/profiles/unknown/time-windows", "", token)
	expectCode(t, "get tw unknown profile", w.Code, 200, w.Body.String())
}

// ================================================================
// 11. History & Stats
// ================================================================

func TestIntegration_HistoryStats(t *testing.T) {
	_, r, token := setupIntegration(t)

	// History
	w := authRequest(r, "GET", "/api/v1/profiles/test/history", "", token)
	expectCode(t, "get history", w.Code, 200, w.Body.String())

	// Stats
	w = authRequest(r, "GET", "/api/v1/profiles/test/stats", "", token)
	expectCode(t, "get stats", w.Code, 200, w.Body.String())
}

// ================================================================
// 12. DNS Query Simulation
// ================================================================

func TestIntegration_QuerySimulation(t *testing.T) {
	_, r, token := setupIntegration(t)

	// With domain
	w := authRequest(r, "GET", "/api/v1/profiles/test/query?domain=pornhub.com", "", token)
	expectCode(t, "simulate query with domain", w.Code, 200, w.Body.String())

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["original_domain"] != "pornhub.com" {
		t.Errorf("expected original_domain=pornhub.com, got %v", resp["original_domain"])
	}
	// CAT_001 (porn) should be blocked → decision 1
	if resp["decision"] != float64(1) {
		t.Errorf("expected blocked decision(1), got %v", resp["decision"])
	}

	// Without domain
	w = authRequest(r, "GET", "/api/v1/profiles/test/query", "", token)
	expectCode(t, "simulate query no domain", w.Code, 400, w.Body.String())

	// Safe domain should be allowed
	w = authRequest(r, "GET", "/api/v1/profiles/test/query?domain=google.com", "", token)
	expectCode(t, "simulate safe domain", w.Code, 200, w.Body.String())
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["decision"] != float64(0) {
		t.Errorf("expected allow decision(0) for google.com, got %v", resp["decision"])
	}
}

// ================================================================
// 13. Payment Endpoints
// ================================================================

func TestIntegration_PaymentMethods(t *testing.T) {
	_, r, token := setupIntegration(t)

	// Stripe plans (public)
	w := request(r, "GET", "/api/v1/stripe/plans", "")
	expectCode(t, "stripe plans public", w.Code, 200, w.Body.String())

	// Stripe webhook (public)
	w = request(r, "POST", "/api/v1/stripe/webhook", `{"type":"mock.unhandled","data":{"object":{}}}`)
	expectCode(t, "stripe webhook", w.Code, 200, w.Body.String())

	// Stripe mock success (public)
	w = request(r, "GET", "/api/v1/stripe/mock-success", "")
	expectCode(t, "stripe mock success", w.Code, 200, w.Body.String())

	// Payment create-order (auth required)
	w = authRequest(r, "POST", "/api/v1/payment/create-order",
		`{"plan_id":"pro","period":"monthly"}`, token)
	expectCode(t, "create payment order", w.Code, 200, w.Body.String())

	// Payment create-order (unauth)
	w = request(r, "POST", "/api/v1/payment/create-order",
		`{"plan_id":"pro","period":"monthly"}`)
	expectCode(t, "create payment order unauth", w.Code, 401, w.Body.String())

	// Payment notify (public)
	w = request(r, "POST", "/api/v1/payment/wechat/notify", `{}`)
	expectCode(t, "payment wechat notify", w.Code, 200, w.Body.String())

	// Mock pay
	w = request(r, "GET", "/api/v1/payment/mock-pay?order=test-order-123", "")
	expectCode(t, "mock pay", w.Code, 200, w.Body.String())

	// Mock pay without order
	w = request(r, "GET", "/api/v1/payment/mock-pay", "")
	expectCode(t, "mock pay no order", w.Code, 400, w.Body.String())
}

// ================================================================
// 14. Stripe (auth required)
// ================================================================

func TestIntegration_StripeAuth(t *testing.T) {
	_, r, token := setupIntegration(t)

	// Create checkout session
	w := authRequest(r, "POST", "/api/v1/stripe/create-checkout-session",
		`{"plan_id":"pro","period":"monthly"}`, token)
	expectCode(t, "stripe create checkout", w.Code, 200, w.Body.String())

	// Subscription status
	w = authRequest(r, "GET", "/api/v1/stripe/subscription-status", "", token)
	expectCode(t, "stripe sub status", w.Code, 200, w.Body.String())

	// Cancel subscription
	w = authRequest(r, "POST", "/api/v1/stripe/cancel-subscription", "", token)
	expectCode(t, "stripe cancel sub", w.Code, 200, w.Body.String())

	// Unauth
	w = request(r, "POST", "/api/v1/stripe/create-checkout-session",
		`{"plan_id":"pro","period":"monthly"}`)
	expectCode(t, "stripe create checkout unauth", w.Code, 401, w.Body.String())

	w = request(r, "GET", "/api/v1/stripe/subscription-status", "")
	expectCode(t, "stripe sub status unauth", w.Code, 401, w.Body.String())

	w = request(r, "POST", "/api/v1/stripe/cancel-subscription", "")
	expectCode(t, "stripe cancel unauth", w.Code, 401, w.Body.String())
}

// ================================================================
// 15. Complete User Flow Integration
// ================================================================

func TestIntegration_FullUserFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	repo := repository.NewTestRepository(dir)
	eng := engine.NewEngine()
	handler := NewAPIHandler(repo, eng)

	r := gin.New()
	r.GET("/login", func(c *gin.Context) { c.String(200, "login page") })
	r.GET("/dashboard", func(c *gin.Context) { c.String(200, "dashboard page") })
	r.GET("/pricing", func(c *gin.Context) { c.String(200, "pricing page") })

	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/register", handler.Register)
		v1.POST("/auth/login", handler.Login)
		v1.GET("/plans", handler.GetPlans)

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

	// ── Step 1: View pages before login ──
	w := request(r, "GET", "/login", "")
	expectCode(t, "flow: login page", w.Code, 200, w.Body.String())

	w = request(r, "GET", "/pricing", "")
	expectCode(t, "flow: pricing page", w.Code, 200, w.Body.String())

	// ── Step 2: Register ──
	w = request(r, "POST", "/api/v1/auth/register",
		`{"email":"fullflow@test.com","password":"secure123"}`)
	expectCode(t, "flow: register", w.Code, 200, w.Body.String())

	var regResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &regResp)
	token, _ := regResp["token"].(string)
	if token == "" {
		t.Fatal("flow: empty token after register")
	}
	t.Logf("flow: registered user, got token")

	// ── Step 3: Dashboard (authed) ──
	w = request(r, "GET", "/dashboard", "")
	expectCode(t, "flow: dashboard page", w.Code, 200, w.Body.String())

	// ── Step 4: Get plans ──
	w = request(r, "GET", "/api/v1/plans", "")
	expectCode(t, "flow: get plans", w.Code, 200, w.Body.String())

	// ── Step 5: View profile ──
	w = authRequest(r, "GET", "/api/v1/user/profile", "", token)
	expectCode(t, "flow: user profile", w.Code, 200, w.Body.String())

	var profileResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &profileResp)
	userObj, _ := profileResp["user"].(map[string]interface{})
	if userObj["email"] != "fullflow@test.com" {
		t.Errorf("flow: expected email fullflow@test.com, got %v", userObj["email"])
	}
	userID := fmt.Sprint(userObj["id"])
	t.Logf("flow: userID=%s", userID)

	// ── Step 6: Subscribe ──
	w = authRequest(r, "GET", "/api/v1/user/subscription", "", token)
	expectCode(t, "flow: subscription", w.Code, 200, w.Body.String())

	// ── Step 7: List profiles ──
	w = authRequest(r, "GET", "/api/v1/profiles", "", token)
	expectCode(t, "flow: list profiles", w.Code, 200, w.Body.String())

	// ── Step 8: Get parental control config ──
	w = authRequest(r, "GET", "/api/v1/profiles/"+userID+"/parental-control", "", token)
	expectCode(t, "flow: parental control", w.Code, 200, w.Body.String())

	// ── Step 9: Toggle categories ──
	w = authRequest(r, "PATCH", "/api/v1/profiles/"+userID+"/parental-control/categories",
		`{"CAT_003":true,"CAT_005":true}`, token)
	expectCode(t, "flow: toggle categories", w.Code, 200, w.Body.String())

	// ── Step 10: Update settings ──
	w = authRequest(r, "PATCH", "/api/v1/profiles/"+userID+"/settings",
		`{"safe_search":true,"phishing_protection":true,"crypto_jacking_protection":true}`, token)
	expectCode(t, "flow: update settings", w.Code, 200, w.Body.String())

	// ── Step 11: Manage allowlist ──
	w = authRequest(r, "POST", "/api/v1/profiles/"+userID+"/allowlist",
		`{"domain":"khanacademy.org","allowed":true}`, token)
	expectCode(t, "flow: add allowlist", w.Code, 200, w.Body.String())

	w = authRequest(r, "GET", "/api/v1/profiles/"+userID+"/allowlist", "", token)
	expectCode(t, "flow: get allowlist", w.Code, 200, w.Body.String())

	// ── Step 12: Manage denylist ──
	w = authRequest(r, "POST", "/api/v1/profiles/"+userID+"/denylist",
		`{"domain":"bad-site.com","reason":"test block"}`, token)
	expectCode(t, "flow: add denylist", w.Code, 200, w.Body.String())

	// ── Step 13: Add time window ──
	w = authRequest(r, "POST", "/api/v1/profiles/"+userID+"/time-windows",
		`{"name":"Weekend Gaming","days_of_week":[0,6],"start_time":"14:00","end_time":"18:00","timezone":"Asia/Shanghai","target_type":"CATEGORY","target_value":"CAT_005"}`, token)
	expectCode(t, "flow: add time window", w.Code, 200, w.Body.String())

	// ── Step 14: Query simulation ──
	w = authRequest(r, "GET", "/api/v1/profiles/"+userID+"/query?domain=test-porn.com", "", token)
	expectCode(t, "flow: simulate query", w.Code, 200, w.Body.String())

	var queryResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &queryResp)
	if queryResp["original_domain"] != "test-porn.com" {
		t.Errorf("flow: expected test-porn.com, got %v", queryResp["original_domain"])
	}
	t.Logf("flow: query decision=%v reason=%v", queryResp["decision"], queryResp["reason"])

	// ── Step 15: View history and stats ──
	w = authRequest(r, "GET", "/api/v1/profiles/user_child_001/history", "", token)
	expectCode(t, "flow: history", w.Code, 200, w.Body.String())

	w = authRequest(r, "GET", "/api/v1/profiles/user_child_001/stats", "", token)
	expectCode(t, "flow: stats", w.Code, 200, w.Body.String())
}

// ================================================================
// 16. CORS headers
// ================================================================

func TestIntegration_CORS(t *testing.T) {
	_, r, _ := setupIntegration(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/api/v1/plans", nil)
	req.Header.Set("Origin", "http://example.com")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}

// ================================================================
// 17. Error handling — 404 routes
// ================================================================

func TestIntegration_NotFound(t *testing.T) {
	_, r, _ := setupIntegration(t)

	w := request(r, "GET", "/api/v1/nonexistent", "")
	expectCode(t, "nonexistent route", w.Code, 404, w.Body.String())
}

// ================================================================
// 18. Login page Enter-key flow integration
// ================================================================

func TestIntegration_AuthRegisterReturnsToken(t *testing.T) {
	_, r, _ := setupIntegration(t)

	w := request(r, "POST", "/api/v1/auth/register",
		`{"email":"checktoken@test.com","password":"pass1234"}`)
	expectCode(t, "register token check", w.Code, 200, w.Body.String())

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["token"] == "" {
		t.Error("expected non-empty token in register response")
	}
	if resp["user"] == nil {
		t.Error("expected user in register response")
	}
	if resp["subscription"] == nil {
		t.Error("expected subscription in register response")
	}
}
