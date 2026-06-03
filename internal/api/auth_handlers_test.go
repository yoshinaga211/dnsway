package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dnsway-pc/internal/auth"
	"dnsway-pc/internal/engine"
	"dnsway-pc/internal/repository"

	"github.com/gin-gonic/gin"
)

func setupTestAuth(t *testing.T) (*APIHandler, *gin.Engine) {
	t.Helper()
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
		v1.GET("/plans", handler.GetPlans)
	}
	return handler, r
}

// ================================================================
// Register
// ================================================================

func TestRegister_Success(t *testing.T) {
	_, r := setupTestAuth(t)
	w := request(r, "POST", "/api/v1/auth/register", `{"email":"newuser@test.com","password":"pass123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["token"] == "" {
		t.Fatal("expected non-empty token")
	}
	if resp["user"] == nil {
		t.Fatal("expected user object")
	}
	if resp["subscription"] == nil {
		t.Fatal("expected subscription object")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	_, r := setupTestAuth(t)
	body := `{"email":"dup@test.com","password":"pass123"}`
	w1 := request(r, "POST", "/api/v1/auth/register", body)
	if w1.Code != http.StatusOK {
		t.Fatalf("first register: expected 200, got %d: %s", w1.Code, w1.Body.String())
	}
	w2 := request(r, "POST", "/api/v1/auth/register", body)
	if w2.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestRegister_MissingEmail(t *testing.T) {
	_, r := setupTestAuth(t)
	w := request(r, "POST", "/api/v1/auth/register", `{"password":"pass123"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegister_MissingPassword(t *testing.T) {
	_, r := setupTestAuth(t)
	w := request(r, "POST", "/api/v1/auth/register", `{"email":"test@test.com"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	_, r := setupTestAuth(t)
	w := request(r, "POST", "/api/v1/auth/register", `{"email":"short@test.com","password":"123"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegister_InvalidJSON(t *testing.T) {
	_, r := setupTestAuth(t)
	w := request(r, "POST", "/api/v1/auth/register", `not-json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ================================================================
// Login
// ================================================================

func TestLogin_Success(t *testing.T) {
	_, r := setupTestAuth(t)
	request(r, "POST", "/api/v1/auth/register", `{"email":"login-test@test.com","password":"pass123"}`)
	w := request(r, "POST", "/api/v1/auth/login", `{"email":"login-test@test.com","password":"pass123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["token"] == "" {
		t.Fatal("expected non-empty token on login")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	_, r := setupTestAuth(t)
	request(r, "POST", "/api/v1/auth/register", `{"email":"wrongpass@test.com","password":"pass123"}`)
	w := request(r, "POST", "/api/v1/auth/login", `{"email":"wrongpass@test.com","password":"wrongpassword"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogin_NoExistingUser(t *testing.T) {
	_, r := setupTestAuth(t)
	w := request(r, "POST", "/api/v1/auth/login", `{"email":"nobody@test.com","password":"pass123"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogin_MissingFields(t *testing.T) {
	_, r := setupTestAuth(t)
	w := request(r, "POST", "/api/v1/auth/login", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	_, r := setupTestAuth(t)
	w := request(r, "POST", "/api/v1/auth/login", `not-json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ================================================================
// Auth-protected endpoints
// ================================================================

func setupTestWithAuthRoutes(t *testing.T) (*APIHandler, *gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	repo := repository.NewTestRepository(dir)
	eng := engine.NewEngine()
	handler := NewAPIHandler(repo, eng)

	rg := gin.New()
	v1 := rg.Group("/api/v1")
	{
		v1.POST("/auth/register", handler.Register)
		v1.POST("/auth/login", handler.Login)

		userGroup := v1.Group("/user")
		userGroup.Use(auth.AuthMiddleware())
		{
			userGroup.GET("/profile", handler.GetUserProfile)
			userGroup.GET("/subscription", handler.GetUserSubscription)
		}
	}

	// Register to get a token
	w := request(rg, "POST", "/api/v1/auth/register", `{"email":"authtest2@test.com","password":"pass123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("register for auth: %s", w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	token := resp["token"].(string)

	return handler, rg, token
}

func authRequest(r *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	return w
}

func TestGetUserProfile_Authenticated(t *testing.T) {
	_, r, token := setupTestWithAuthRoutes(t)
	w := authRequest(r, "GET", "/api/v1/user/profile", "", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["user"] == nil {
		t.Fatal("expected user in response")
	}
}

func TestGetUserProfile_Unauthenticated(t *testing.T) {
	_, r, _ := setupTestWithAuthRoutes(t)
	w := request(r, "GET", "/api/v1/user/profile", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetUserProfile_InvalidToken(t *testing.T) {
	_, r, _ := setupTestWithAuthRoutes(t)
	w := authRequest(r, "GET", "/api/v1/user/profile", "", "invalid-token")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetUserSubscription_Authenticated(t *testing.T) {
	_, r, token := setupTestWithAuthRoutes(t)
	w := authRequest(r, "GET", "/api/v1/user/subscription", "", token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetUserSubscription_Unauthenticated(t *testing.T) {
	_, r, _ := setupTestWithAuthRoutes(t)
	w := request(r, "GET", "/api/v1/user/subscription", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// ================================================================
// Plans
// ================================================================

func TestGetPlans(t *testing.T) {
	_, r := setupTestAuth(t)
	w := request(r, "GET", "/api/v1/plans", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["plans"] == nil {
		t.Fatal("expected plans in response")
	}
}
