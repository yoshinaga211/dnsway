package payment

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"dnsway-pc/internal/models"
)

func mchID() string { return os.Getenv("WECHAT_MCH_ID") }
func apiKey() string { return os.Getenv("WECHAT_API_KEY") }
func isMock() bool   { return mchID() == "" || apiKey() == "" }

func init() {
	rand.Seed(time.Now().UnixNano())
}

// CreateOrder generates a WeChat Pay order (JSAPI or NATIVE). In mock mode, auto-confirms.
func CreateOrder(userID, planID, period string, planPrice int, notifyURL string) (*models.OrderResponse, error) {
	amount := planPrice
	desc := fmt.Sprintf("DNS.1 %s - %s", planID, period)
	orderID := fmt.Sprintf("DNS1%d%06d", time.Now().Unix(), rand.Intn(999999))

	if isMock() {
		return &models.OrderResponse{
			OrderID:     orderID,
			PaymentURL:  fmt.Sprintf("/api/v1/payment/mock-pay?order=%s", orderID),
			Amount:      amount,
			Currency:    "CNY",
			Description: desc,
		}, nil
	}

	// Real WeChat Pay V2 UnifiedOrder API
	// Returns prepay_id for JSAPI or code_url for NATIVE
	return &models.OrderResponse{
		OrderID:     orderID,
		Amount:      amount,
		Currency:    "CNY",
		Description: desc,
	}, fmt.Errorf("real WeChat Pay requires server IP whitelist and merchant cert (not configured)")
}

// HandlePaymentNotify processes the WeChat Pay callback XML.
func HandlePaymentNotify(xmlBody []byte) (*models.PaymentRecord, error) {
	if isMock() {
		return &models.PaymentRecord{
			Status:   "paid",
			PaidAt:   time.Now().UTC().Format(time.RFC3339),
			Provider: "wechat",
		}, nil
	}
	// Parse XML, verify signature, update order status
	return nil, fmt.Errorf("real notify handling not configured")
}

// MockConfirmPayment simulates a successful payment for development.
func MockConfirmPayment(orderID string) *models.PaymentRecord {
	return &models.PaymentRecord{
		ProviderOrderID: orderID,
		Status:          "paid",
		Provider:        "wechat",
		PaidAt:          time.Now().UTC().Format(time.RFC3339),
	}
}
