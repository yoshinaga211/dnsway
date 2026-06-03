package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"dnsway-pc/internal/auth"
	"dnsway-pc/internal/models"
	"dnsway-pc/internal/payment"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/subscription"
)

func baseURL() string {
	u := os.Getenv("BASE_URL")
	if u == "" {
		u = "http://localhost:8081"
	}
	return u
}

// POST /api/v1/stripe/create-checkout-session
func (h *APIHandler) CreateStripeCheckoutSession(c *gin.Context) {
	claims := auth.GetClaimsFromContext(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED"})
		return
	}

	var req models.CheckoutSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_INPUT", "details": "plan_id and period required"})
		return
	}

	userRow, err := h.repo.GetUserByID(claims.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "USER_NOT_FOUND"})
		return
	}

	email := fmt.Sprint(userRow["email"])
	customerID := ""
	if cid, ok := userRow["stripe_customer_id"].(string); ok && cid != "" {
		customerID = cid
	}

	successURL := baseURL() + "/payment/success"
	cancelURL := baseURL() + "/payment/cancel"

	sessionID, url, err := payment.CreateCheckoutSession(email, customerID, claims.UserID, req.PlanID, req.Period, successURL, cancelURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "STRIPE_ERROR", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.CheckoutSessionResponse{
		SessionID: sessionID,
		URL:       url,
	})
}

// POST /api/v1/stripe/webhook
func (h *APIHandler) StripeWebhook(c *gin.Context) {
	payload, _ := c.GetRawData()
	sigHeader := c.GetHeader("Stripe-Signature")

	event, err := payment.ValidateWebhookSignature(payload, sigHeader)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "WEBHOOK_SIGNATURE_INVALID"})
		return
	}

	// Handle mock mode
	if event.Type == "mock.unhandled" {
		var generic map[string]interface{}
		json.Unmarshal(payload, &generic)
		h.handleStripeEvent(generic)
		c.JSON(http.StatusOK, gin.H{"status": "MOCK_PROCESSED"})
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		json.Unmarshal(event.Data.Raw, &session)
		h.handleCheckoutCompleted(&session)
	case "customer.subscription.updated":
		var sub stripe.Subscription
		json.Unmarshal(event.Data.Raw, &sub)
		h.handleSubscriptionUpdated(&sub)
	case "customer.subscription.deleted":
		var sub stripe.Subscription
		json.Unmarshal(event.Data.Raw, &sub)
		h.handleSubscriptionDeleted(&sub)
	case "invoice.paid":
		c.JSON(http.StatusOK, gin.H{"status": "RECEIVED"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "PROCESSED"})
}

func (h *APIHandler) handleStripeEvent(generic map[string]interface{}) {
	eventType, _ := generic["type"].(string)
	switch eventType {
	case "checkout.session.completed":
		if data, ok := generic["data"].(map[string]interface{}); ok {
			if obj, ok := data["object"].(map[string]interface{}); ok {
				customerEmail, _ := obj["customer_email"].(string)
				customerID, _ := obj["customer"].(string)
				subID, _ := obj["subscription"].(string)
				if customerEmail != "" {
					userRow, err := h.repo.GetUserByEmail(customerEmail)
					if err == nil {
						userID := fmt.Sprint(userRow["id"])
						if customerID != "" {
							h.repo.UpdateUserStripeCustomer(userID, customerID)
						}
						h.repo.UpdateSubscriptionStripe(userID, "pro", "active", subID, "")
					}
				}
			}
		}
	}
}

func (h *APIHandler) handleCheckoutCompleted(s *stripe.CheckoutSession) {
	if s.CustomerEmail != "" {
		userRow, err := h.repo.GetUserByEmail(s.CustomerEmail)
		if err == nil {
			userID := fmt.Sprint(userRow["id"])
			if s.Customer != nil {
				h.repo.UpdateUserStripeCustomer(userID, s.Customer.ID)
			}
			// Determine plan from metadata or default
			planID := "pro"
			if s.Metadata != nil {
				if p, ok := s.Metadata["plan_id"]; ok {
					planID = p
				}
			}
			subID := ""
			if s.Subscription != nil {
				subID = s.Subscription.ID
			}
			h.repo.UpdateSubscriptionStripe(userID, planID, "active", subID, "")
		}
	}
}

func (h *APIHandler) handleSubscriptionUpdated(s *stripe.Subscription) {
	if s.Customer != nil {
		userRow, err := h.repo.GetUserByStripeCustomerID(s.Customer.ID)
		if err == nil {
			userID := fmt.Sprint(userRow["id"])
			status := string(s.Status)
			periodEnd := ""
			if s.Items != nil && len(s.Items.Data) > 0 {
				periodEnd = fmt.Sprintf("%d", s.Items.Data[0].CurrentPeriodEnd)
			}
			h.repo.UpdateSubscriptionStripe(userID, "pro", status, s.ID, periodEnd)
		}
	}
}

func (h *APIHandler) handleSubscriptionDeleted(s *stripe.Subscription) {
	if s.Customer != nil {
		userRow, err := h.repo.GetUserByStripeCustomerID(s.Customer.ID)
		if err == nil {
			h.repo.CancelSubscriptionStripe(fmt.Sprint(userRow["id"]))
		}
	}
}

// GET /api/v1/stripe/subscription-status
func (h *APIHandler) GetStripeSubscriptionStatus(c *gin.Context) {
	claims := auth.GetClaimsFromContext(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED"})
		return
	}

	subRow, err := h.repo.GetSubscription(claims.UserID)
	if err != nil {
		c.JSON(http.StatusOK, models.SubscriptionStatusResponse{
			Status: "none",
			Plan:   "free",
		})
		return
	}

	planID := fmt.Sprint(subRow["plan_id"])
	planName := planID
	plans := payment.StripePlans()
	if p, ok := plans[planID]; ok {
		planName = fmt.Sprint(p["name"])
	}

	status := fmt.Sprint(subRow["status"])
	trialEnd := fmt.Sprint(subRow["trial_ends_at"])
	var trialRemaining int
	if status == "trialing" && trialEnd != "" {
		if endTime, err := time.Parse(time.RFC3339, trialEnd); err == nil {
			remaining := time.Until(endTime)
			if remaining > 0 {
				trialRemaining = int(remaining.Hours() / 24)
			}
		}
	}

	c.JSON(http.StatusOK, models.SubscriptionStatusResponse{
		Status:           status,
		Plan:             planID,
		PlanName:         planName,
		CurrentPeriodEnd: fmt.Sprint(subRow["current_period_end"]),
		CancelAtPeriodEnd: false,
		TrialRemaining:   trialRemaining,
		IsTrialing:       status == "trialing",
	})
}

// POST /api/v1/stripe/cancel-subscription
func (h *APIHandler) CancelStripeSubscription(c *gin.Context) {
	claims := auth.GetClaimsFromContext(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED"})
		return
	}

	userRow, err := h.repo.GetUserByID(claims.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "USER_NOT_FOUND"})
		return
	}

	stripeCustomerID := ""
	if cid, ok := userRow["stripe_customer_id"].(string); ok {
		stripeCustomerID = cid
	}

	// Cancel in Stripe if real API is configured
	if payment.IsStripeConfigured() && stripeCustomerID != "" {
		subs := subscription.List(&stripe.SubscriptionListParams{
			Customer: stripe.String(stripeCustomerID),
			Status:   stripe.String("active"),
		})
		for subs.Next() {
			subscription.Cancel(subs.Subscription().ID, nil)
		}
	}

	// Update local subscription
	h.repo.CancelSubscriptionStripe(claims.UserID)

	c.JSON(http.StatusOK, gin.H{"status": "CANCELED"})
}

// GET /api/v1/stripe/plans
func (h *APIHandler) GetStripePlans(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"plans": payment.StripePlans()})
}

// GET /api/v1/stripe/mock-success
func (h *APIHandler) StripeMockSuccess(c *gin.Context) {
	_ = c.Query("session_id")
	plan := c.Query("plan")
	period := c.Query("period")
	if plan == "" {
		plan = "pro"
	}
	if period == "" {
		period = "monthly"
	}

	// If user is authenticated, activate subscription
	claims := auth.GetClaimsFromContext(c)
	userID := c.Query("user_id")
	if claims != nil {
		userID = claims.UserID
	}
	if userID != "" {
		h.repo.ActivateSubscription(userID, plan, period)
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="UTF-8"><title>Payment Success</title>
<style>
:root { --primary: #4f6ef7; --bg: #f7f8fa; --surface: #fff; --border: #e8ecf1; --text: #1a1d26; --muted: #6b7280; --green: #22c55e; }
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: var(--bg); display: flex; align-items: center; justify-content: center; min-height: 100vh; }
.card { background: var(--surface); border: 1px solid var(--border); border-radius: 12px; padding: 40px 32px; text-align: center; max-width: 420px; width: 90vw; box-shadow: 0 1px 3px rgba(0,0,0,0.04); }
.check { width: 64px; height: 64px; background: #dcfce7; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; font-size: 32px; margin-bottom: 16px; }
h2 { font-size: 1.3rem; margin-bottom: 8px; }
p { color: var(--muted); font-size: 0.9rem; margin-bottom: 20px; }
.btn { display: inline-block; padding: 10px 24px; background: var(--primary); color: #fff; border-radius: 8px; text-decoration: none; font-weight: 600; font-size: 0.9rem; }
.btn:hover { opacity: 0.9; }
</style></head><body>
<div class="card">
    <div class="check">✓</div>
    <h2>支付成功！</h2>
    <p>感谢订阅 DNS.1 `+plan+` 方案。您的账号已激活，所有功能已解锁。</p>
    <a href="/dashboard" class="btn">前往控制台</a>
</div>
</body></html>`)
}