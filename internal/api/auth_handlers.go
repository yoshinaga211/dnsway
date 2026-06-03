package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"time"

	"dnsway-pc/internal/auth"
	"dnsway-pc/internal/models"
	"dnsway-pc/internal/payment"

	"github.com/gin-gonic/gin"
)

// ================================================================
// Auth Handlers
// ================================================================

func hashPassword(pw string) string {
	h := sha256.Sum256([]byte(pw))
	return hex.EncodeToString(h[:])
}

func userFromRow(row map[string]interface{}) *models.User {
	if row == nil {
		return nil
	}
	return &models.User{
		ID:             fmt.Sprint(row["id"]),
		Email:          fmt.Sprint(row["email"]),
		WechatUnionID:  fmt.Sprint(row["wechat_unionid"]),
		WechatOpenID:   fmt.Sprint(row["wechat_openid"]),
		WechatNickname: fmt.Sprint(row["wechat_nickname"]),
		WechatAvatar:   fmt.Sprint(row["wechat_avatar"]),
		Status:         fmt.Sprint(row["status"]),
		CreatedAt:      fmt.Sprint(row["created_at"]),
		UpdatedAt:      fmt.Sprint(row["updated_at"]),
	}
}

func subFromRow(row map[string]interface{}) *models.Subscription {
	if row == nil {
		return nil
	}
	return &models.Subscription{
		ID:                 fmt.Sprint(row["id"]),
		UserID:             fmt.Sprint(row["user_id"]),
		PlanID:             fmt.Sprint(row["plan_id"]),
		Status:             fmt.Sprint(row["status"]),
		TrialEndsAt:        fmt.Sprint(row["trial_ends_at"]),
		CurrentPeriodStart: fmt.Sprint(row["current_period_start"]),
		CurrentPeriodEnd:   fmt.Sprint(row["current_period_end"]),
		CreatedAt:          fmt.Sprint(row["created_at"]),
		UpdatedAt:          fmt.Sprint(row["updated_at"]),
	}
}

func trialRemaining(sub *models.Subscription) int {
	if sub == nil || sub.Status != "trialing" || sub.TrialEndsAt == "" {
		return 0
	}
	endTime, err := time.Parse(time.RFC3339, sub.TrialEndsAt)
	if err != nil {
		return 0
	}
	remaining := time.Until(endTime)
	if remaining < 0 {
		return 0
	}
	return int(remaining.Hours() / 24)
}

// POST /api/v1/auth/register
func (h *APIHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_INPUT", "details": "Email and password required"})
		return
	}
	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_INPUT", "details": "Password must be at least 6 characters"})
		return
	}

	// Check if user exists
	existing, _ := h.repo.GetUserByEmail(req.Email)
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "EMAIL_EXISTS", "details": "Email already registered"})
		return
	}

	userID, err := h.repo.CreateUser(req.Email, hashPassword(req.Password))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
		return
	}

	// Create 7-day trial subscription
	_, err = h.repo.CreateSubscription(userID, "free", 7)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
		return
	}

	// Create default profile for this user
	h.repo.UpdateProfileSettings(userID, boolPtr(true), boolPtr(true), boolPtr(false), intPtr(7), boolPtr(false), boolPtr(false), boolPtr(false), boolPtr(false), strPtr("us"))
	h.engine.UpdateUserConfig(userID, "CAT_001", true)
	h.engine.UpdateUserConfig(userID, "CAT_008", true)

	token, err := auth.GenerateToken(userID, req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "TOKEN_ERROR"})
		return
	}

	userRow, _ := h.repo.GetUserByID(userID)
	subRow, _ := h.repo.GetSubscription(userID)
	u := userFromRow(userRow)
	s := subFromRow(subRow)

	c.JSON(http.StatusOK, models.AuthResponse{
		Token:          token,
		User:           u,
		Subscription:   s,
		TrialRemaining: trialRemaining(s),
	})
}

// POST /api/v1/auth/login
func (h *APIHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_INPUT", "details": "Email and password required"})
		return
	}

	row, err := h.repo.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_CREDENTIALS"})
		return
	}

	if fmt.Sprint(row["password_hash"]) != hashPassword(req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_CREDENTIALS"})
		return
	}

	userID := fmt.Sprint(row["id"])
	token, err := auth.GenerateToken(userID, req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "TOKEN_ERROR"})
		return
	}

	subRow, _ := h.repo.GetSubscription(userID)
	u := userFromRow(row)
	s := subFromRow(subRow)

	c.JSON(http.StatusOK, models.AuthResponse{
		Token:          token,
		User:           u,
		Subscription:   s,
		TrialRemaining: trialRemaining(s),
	})
}

// POST /api/v1/auth/wechat/qrcode
func (h *APIHandler) WechatQRCode(c *gin.Context) {
	state := auth.GenerateQRState()
	qrURL := auth.GetQRCodeURL(state)
	c.JSON(http.StatusOK, models.WechatQRResponse{QRURL: qrURL, State: state})
}

// GET /api/v1/auth/wechat/callback?code=&state=
func (h *APIHandler) WechatCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_INPUT", "details": "Missing code"})
		return
	}

	info, err := auth.ExchangeCodeForUser(code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "WECHAT_ERROR", "details": err.Error()})
		return
	}

	// Look up existing user by unionid, or create new
	row, err := h.repo.GetUserByWechatUnionID(info.UnionID)
	var userID, email string
	if err != nil {
		email = fmt.Sprintf("wx_%s@dns1.local", info.UnionID)
		userID, _ = h.repo.CreateUser(email, "")
		h.repo.UpdateUserWechat(userID, info.UnionID, info.OpenID, info.Nickname, info.HeadImgURL)
		h.repo.CreateSubscription(userID, "free", 7)
		h.repo.UpdateProfileSettings(userID, boolPtr(true), boolPtr(true), boolPtr(false), intPtr(7), boolPtr(false), boolPtr(false), boolPtr(false), boolPtr(false), strPtr("us"))
	} else {
		userID = fmt.Sprint(row["id"])
		email = fmt.Sprint(row["email"])
		h.repo.UpdateUserWechat(userID, info.UnionID, info.OpenID, info.Nickname, info.HeadImgURL)
		if _, err := h.repo.GetSubscription(userID); err != nil {
			h.repo.CreateSubscription(userID, "free", 7)
		}
	}

	token, err := auth.GenerateToken(userID, email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "TOKEN_ERROR"})
		return
	}

	// Return HTML page that stores token and redirects
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="UTF-8"></head><body>
<script>
localStorage.setItem('dns1_token', '%s');
window.location.href = '/dashboard';
</script>
<p>登录成功，正在跳转...</p></body></html>`, token))
}

// GET /api/v1/user/profile
func (h *APIHandler) GetUserProfile(c *gin.Context) {
	claims := auth.GetClaimsFromContext(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED"})
		return
	}
	row, err := h.repo.GetUserByID(claims.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "USER_NOT_FOUND"})
		return
	}
	u := userFromRow(row)
	subRow, _ := h.repo.GetSubscription(claims.UserID)
	s := subFromRow(subRow)
	c.JSON(http.StatusOK, gin.H{
		"user":              u,
		"subscription":      s,
		"trial_remaining_days": trialRemaining(s),
	})
}

// GET /api/v1/user/subscription
func (h *APIHandler) GetUserSubscription(c *gin.Context) {
	claims := auth.GetClaimsFromContext(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED"})
		return
	}
	subRow, err := h.repo.GetSubscription(claims.UserID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"subscription": nil, "plan": "free", "status": "none"})
		return
	}
	s := subFromRow(subRow)
	planRow, _ := h.repo.GetPlan(s.PlanID)
	planName := "Free"
	if planRow != nil {
		planName = fmt.Sprint(planRow["name"])
	}
	c.JSON(http.StatusOK, gin.H{
		"subscription":        s,
		"plan":                planName,
		"trial_remaining_days": trialRemaining(s),
	})
}

// ================================================================
// Plan & Payment Handlers
// ================================================================

// GET /api/v1/plans
func (h *APIHandler) GetPlans(c *gin.Context) {
	plans, err := h.repo.GetPlans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"plans": plans})
}

// POST /api/v1/payment/create-order
func (h *APIHandler) CreatePaymentOrder(c *gin.Context) {
	claims := auth.GetClaimsFromContext(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED"})
		return
	}

	var req models.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_INPUT", "details": "plan_id and period required"})
		return
	}

	planRow, err := h.repo.GetPlan(req.PlanID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "PLAN_NOT_FOUND"})
		return
	}

	price := 0
	if req.Period == "yearly" {
		price = planRow["price_yearly"].(int)
	} else {
		price = planRow["price_monthly"].(int)
	}

	if price == 0 {
		// Free plan: activate immediately
		h.repo.ActivateSubscription(claims.UserID, req.PlanID, req.Period)
		c.JSON(http.StatusOK, gin.H{"status": "ACTIVATED", "plan": req.PlanID})
		return
	}

	notifyURL := osGetenv("BASE_URL", "http://localhost:8081") + "/api/v1/payment/wechat/notify"
	order, err := payment.CreateOrder(claims.UserID, req.PlanID, req.Period, price, notifyURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "PAYMENT_ERROR", "details": err.Error()})
		return
	}

	// Save payment record
	h.repo.CreatePaymentRecord(claims.UserID, req.PlanID, "wechat", order.OrderID, price)

	c.JSON(http.StatusOK, order)
}

// POST /api/v1/payment/wechat/notify
func (h *APIHandler) WechatPaymentNotify(c *gin.Context) {
	body, _ := c.GetRawData()
	record, err := payment.HandlePaymentNotify(body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "NOTIFY_ERROR"})
		return
	}
	if record != nil && record.Status == "paid" {
		h.repo.ConfirmPayment(record.ProviderOrderID)
	}
	c.String(http.StatusOK, `<xml><return_code><![CDATA[SUCCESS]]></return_code></xml>`)
}

// GET /api/v1/payment/mock-pay
func (h *APIHandler) MockPayConfirm(c *gin.Context) {
	orderID := c.Query("order")
	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing order ID"})
		return
	}
	h.repo.ConfirmPayment(orderID)
	payRow, _ := h.repo.GetPaymentByOrderID(orderID)
	if payRow != nil {
		h.repo.ActivateSubscription(fmt.Sprint(payRow["user_id"]), fmt.Sprint(payRow["plan_id"]), "monthly")
	}
	c.JSON(http.StatusOK, gin.H{"status": "PAID", "order_id": orderID})
}

// ================================================================
// Helpers
// ================================================================

func boolPtr(b bool) *bool    { return &b }
func intPtr(i int) *int       { return &i }
func strPtr(s string) *string { return &s }

func osGetenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
