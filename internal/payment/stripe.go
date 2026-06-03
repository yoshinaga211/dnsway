package payment

import (
	"fmt"
	"os"

	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/checkout/session"
	"github.com/stripe/stripe-go/v84/webhook"
)

var stripeKey string

func InitStripe() {
	stripeKey = os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		stripeKey = "sk_test_mock"
	}
	stripe.Key = stripeKey
}

// IsStripeConfigured returns true when a real API key is set.
func IsStripeConfigured() bool {
	k := os.Getenv("STRIPE_SECRET_KEY")
	return k != "" && !isMockStripe() && k != "sk_test_mock"
}

func isMockStripe() bool {
	k := os.Getenv("STRIPE_SECRET_KEY")
	return k == "" || k == "sk_test_mock"
}

// StripePlans returns the product definitions for Stripe.
func StripePlans() map[string]map[string]interface{} {
	return map[string]map[string]interface{}{
		"pro": {
			"name":             "Pro",
			"description":      "全功能解锁，无限配置",
			"price_monthly":    2999, // ¥29.99
			"price_yearly":     23900, // ¥239.00（省33%）
			"price_id_monthly": os.Getenv("STRIPE_PRICE_PRO_MONTHLY"),
			"price_id_yearly":  os.Getenv("STRIPE_PRICE_PRO_YEARLY"),
			"features":         []string{"无限配置文件", "AI 威胁检测", "加密挖矿防护", "钓鱼域名拦截", "可疑 TLD 拦截", "绕过预防"},
		},
	}
}

// CreateCheckoutSession creates a Stripe Checkout session.
func CreateCheckoutSession(customerEmail, customerID, userID, planID, period, successURL, cancelURL string) (string, string, error) {
	if isMockStripe() {
		sessionID := fmt.Sprintf("cs_mock_%d", os.Getpid())
		return sessionID, fmt.Sprintf("/api/v1/stripe/mock-success?session_id=%s&plan=%s&period=%s&user_id=%s", sessionID, planID, period, userID), nil
	}

	plans := StripePlans()
	plan, ok := plans[planID]
	if !ok {
		return "", "", fmt.Errorf("unknown plan: %s", planID)
	}

	var priceID string
	if period == "yearly" {
		priceID = fmt.Sprint(plan["price_id_yearly"])
	} else {
		priceID = fmt.Sprint(plan["price_id_monthly"])
	}

	params := &stripe.CheckoutSessionParams{
		CustomerEmail: stripe.String(customerEmail),
		Mode:          stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:    stripe.String(successURL + "?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:     stripe.String(cancelURL),
		AutomaticTax: &stripe.CheckoutSessionAutomaticTaxParams{Enabled: stripe.Bool(false)},
	}

	if customerID != "" {
		params.Customer = stripe.String(customerID)
	}

	if priceID != "" {
		params.LineItems = []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		}
	} else {
		// Create a one-time line item if no price ID is configured
		amount := int64(plan["price_monthly"].(int))
		if period == "yearly" {
			amount = int64(plan["price_yearly"].(int))
		}
		params.LineItems = []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(fmt.Sprintf("DNS.1 %s (%s)", plan["name"], period)),
					},
					Recurring: &stripe.CheckoutSessionLineItemPriceDataRecurringParams{
						Interval: stripe.String(string(stripe.PriceRecurringIntervalMonth)),
					},
					UnitAmount: stripe.Int64(amount),
				},
				Quantity: stripe.Int64(1),
			},
		}
		if period == "yearly" {
			params.LineItems[0].PriceData.Recurring.Interval = stripe.String(string(stripe.PriceRecurringIntervalYear))
		}
	}

	s, err := session.New(params)
	if err != nil {
		return "", "", fmt.Errorf("stripe session create: %w", err)
	}
	return s.ID, s.URL, nil
}

// ValidateWebhookSignature verifies the Stripe webhook signature.
func ValidateWebhookSignature(payload []byte, sigHeader string) (stripe.Event, error) {
	if isMockStripe() {
		return stripe.Event{Type: "mock.unhandled"}, nil
	}
	endpointSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if endpointSecret == "" {
		return stripe.Event{}, fmt.Errorf("STRIPE_WEBHOOK_SECRET not set")
	}
	return webhook.ConstructEvent(payload, sigHeader, endpointSecret)
}

// MockConfirmStripePayment simulates a Stripe payment for development.
func MockConfirmStripePayment(sessionID string) (string, string, string) {
	return sessionID, "active", "pro"
}
