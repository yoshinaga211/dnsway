package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dnsway-pc/internal/engine"
	"dnsway-pc/internal/repository"

	"github.com/gin-gonic/gin"
)

func setupTest(t *testing.T) (*APIHandler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	repo := repository.NewTestRepository(dir)
	eng := engine.NewEngine()
	handler := NewAPIHandler(repo, eng)

	r := gin.New()
	// Register handler routes
	rg := r.Group("/api/v1")
	{
		rg.GET("/profiles", handler.ListProfiles)
		rg.GET("/profiles/:profileId/config", handler.GetParentalControl)
		rg.PUT("/profiles/:profileId/categories", handler.UpdateCategories)
		rg.PUT("/profiles/:profileId/settings", handler.UpdateSettings)

		rg.GET("/profiles/:profileId/allowlist", handler.GetAllowlist)
		rg.POST("/profiles/:profileId/allowlist", handler.UpdateAllowlist)
		rg.DELETE("/profiles/:profileId/allowlist/:domain", handler.DeleteAllowlist)

		rg.GET("/profiles/:profileId/denylist", handler.GetDenylist)
		rg.POST("/profiles/:profileId/denylist", handler.UpdateDenylist)
		rg.DELETE("/profiles/:profileId/denylist/:domain", handler.DeleteDenylist)

		rg.GET("/profiles/:profileId/time-windows", handler.GetTimeWindows)
		rg.POST("/profiles/:profileId/time-windows", handler.AddTimeWindow)
		rg.DELETE("/profiles/:profileId/time-windows/:id", handler.DeleteTimeWindow)

		rg.GET("/profiles/:profileId/history", handler.GetHistory)
		rg.GET("/profiles/:profileId/stats", handler.GetStats)
		rg.GET("/profiles/:profileId/simulate", handler.SimulateQuery)
	}
	return handler, r
}

func request(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// ================================================================
// Profile Listing
// ================================================================

func TestListProfiles(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "GET", "/api/v1/profiles", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	profiles, ok := resp["profiles"].([]interface{})
	if !ok {
		t.Fatalf("expected profiles array, got %T", resp["profiles"])
	}
	if len(profiles) < 2 {
		t.Fatalf("expected at least 2 profiles, got %d", len(profiles))
	}
}

// ================================================================
// Parental Control Config
// ================================================================

func TestGetParentalControlOK(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "GET", "/api/v1/profiles/test/config", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["profile_id"] != "test" {
		t.Fatalf("expected profile_id=test, got %v", resp["profile_id"])
	}
}

func TestGetParentalControlNotFound(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "GET", "/api/v1/profiles/nonexistent/config", "")
	// Should return a 200 with empty config (repo returns empty map for unknown profiles)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ================================================================
// Categories
// ================================================================

func TestUpdateCategories(t *testing.T) {
	_, r := setupTest(t)
	body := `{"CAT_001": true, "CAT_002": false}`
	w := request(r, "PUT", "/api/v1/profiles/test/categories", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %v", resp["status"])
	}
}

func TestUpdateCategoriesInvalidBody(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "PUT", "/api/v1/profiles/test/categories", `not-json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ================================================================
// Settings
// ================================================================

func TestUpdateSettings(t *testing.T) {
	_, r := setupTest(t)
	body := `{"safe_search": false, "log_retention_days": 14}`
	w := request(r, "PUT", "/api/v1/profiles/test/settings", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %v", resp["status"])
	}
}

func TestUpdateSettingsInvalidBody(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "PUT", "/api/v1/profiles/test/settings", `not-json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ================================================================
// Allowlist
// ================================================================

func TestGetAllowlist(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "GET", "/api/v1/profiles/test/allowlist", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	list, ok := resp["allowlist"].([]interface{})
	if !ok {
		t.Fatalf("expected allowlist array, got %T", resp["allowlist"])
	}
	if len(list) == 0 {
		t.Fatal("expected non-empty allowlist")
	}
}

func TestGetAllowlistEmptyProfile(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "GET", "/api/v1/profiles/unknown/allowlist", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateAllowlistAdd(t *testing.T) {
	_, r := setupTest(t)
	body := `{"domain": "example.org", "allowed": true}`
	w := request(r, "POST", "/api/v1/profiles/test/allowlist", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %v", resp["status"])
	}
}

func TestUpdateAllowlistRemove(t *testing.T) {
	_, r := setupTest(t)
	body := `{"domain": "khanacademy.org", "allowed": false}`
	w := request(r, "POST", "/api/v1/profiles/test/allowlist", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateAllowlistInvalidBody(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "POST", "/api/v1/profiles/test/allowlist", `bad`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteAllowlist(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "DELETE", "/api/v1/profiles/test/allowlist/khanacademy.org", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ================================================================
// Denylist
// ================================================================

func TestGetDenylist(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "GET", "/api/v1/profiles/test/denylist", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetDenylistEmptyProfile(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "GET", "/api/v1/profiles/unknown/denylist", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateDenylist(t *testing.T) {
	_, r := setupTest(t)
	body := `{"domain": "evil.com", "reason": "test"}`
	w := request(r, "POST", "/api/v1/profiles/test/denylist", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %v", resp["status"])
	}
}

func TestUpdateDenylistInvalidBody(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "POST", "/api/v1/profiles/test/denylist", `bad`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteDenylist(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "DELETE", "/api/v1/profiles/test/denylist/malicious-site.net", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ================================================================
// Time Windows
// ================================================================

func TestGetTimeWindows(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "GET", "/api/v1/profiles/test/time-windows", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetTimeWindowsEmptyProfile(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "GET", "/api/v1/profiles/unknown/time-windows", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddTimeWindow(t *testing.T) {
	_, r := setupTest(t)
	body := `{
		"name": "Test Block",
		"days_of_week": [1,2,3],
		"start_time": "09:00",
		"end_time": "17:00",
		"timezone": "UTC",
		"target_type": "DOMAIN",
		"target_value": "games.example.com"
	}`
	w := request(r, "POST", "/api/v1/profiles/test/time-windows", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddTimeWindowInvalidBody(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "POST", "/api/v1/profiles/test/time-windows", `bad`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteTimeWindow(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "DELETE", "/api/v1/profiles/test/time-windows/tw-default", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ================================================================
// History & Stats
// ================================================================

func TestGetHistory(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "GET", "/api/v1/profiles/test/history", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetStats(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "GET", "/api/v1/profiles/test/stats", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ================================================================
// Query Simulation
// ================================================================

func TestSimulateQuery(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "GET", "/api/v1/profiles/test/simulate?domain=example.com", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["original_domain"] != "example.com" {
		t.Fatalf("expected original_domain=example.com, got %v", resp["original_domain"])
	}
}

func TestSimulateQueryNoDomain(t *testing.T) {
	_, r := setupTest(t)
	w := request(r, "GET", "/api/v1/profiles/test/simulate", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
