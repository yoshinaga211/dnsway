package payment

import (
	"os"
	"testing"
)

func TestStripePlans(t *testing.T) {
	plans := StripePlans()
	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}
	pro, ok := plans["pro"]
	if !ok {
		t.Fatal("expected 'pro' plan")
	}
	if pro["price_monthly"] != 499 {
		t.Fatalf("expected pro monthly 499, got %v", pro["price_monthly"])
	}
	if pro["price_yearly"] != 3999 {
		t.Fatalf("expected pro yearly 3999, got %v", pro["price_yearly"])
	}
	family, ok := plans["family"]
	if !ok {
		t.Fatal("expected 'family' plan")
	}
	if family["price_monthly"] != 799 {
		t.Fatalf("expected family monthly 799, got %v", family["price_monthly"])
	}
}

func TestIsStripeConfigured_Empty(t *testing.T) {
	os.Unsetenv("STRIPE_SECRET_KEY")
	if IsStripeConfigured() {
		t.Fatal("expected false when STRIPE_SECRET_KEY is unset")
	}
}

func TestIsStripeConfigured_Mock(t *testing.T) {
	os.Setenv("STRIPE_SECRET_KEY", "sk_test_mock")
	defer os.Unsetenv("STRIPE_SECRET_KEY")
	if IsStripeConfigured() {
		t.Fatal("expected false for mock key")
	}
}

func TestIsStripeConfigured_Real(t *testing.T) {
	os.Setenv("STRIPE_SECRET_KEY", "sk_live_real123")
	defer os.Unsetenv("STRIPE_SECRET_KEY")
	if !IsStripeConfigured() {
		t.Fatal("expected true for real key")
	}
}

func TestCreateCheckoutSession_Mock(t *testing.T) {
	sessionID, url, err := CreateCheckoutSession("test@example.com", "", "user1", "pro", "monthly", "https://example.com/success", "https://example.com/cancel")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if url == "" {
		t.Fatal("expected non-empty URL")
	}
}


func TestValidateWebhookSignature_Mock(t *testing.T) {
	event, err := ValidateWebhookSignature([]byte(`{}`), "test-sig")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != "mock.unhandled" {
		t.Fatalf("expected mock.unhandled, got %s", event.Type)
	}
}

func TestMockConfirmStripePayment(t *testing.T) {
	sessionID, status, plan := MockConfirmStripePayment("cs_mock_123")
	if sessionID != "cs_mock_123" {
		t.Fatalf("expected cs_mock_123, got %s", sessionID)
	}
	if status != "active" {
		t.Fatalf("expected active, got %s", status)
	}
	if plan != "pro" {
		t.Fatalf("expected pro, got %s", plan)
	}
}

func TestCreateOrder_Mock(t *testing.T) {
	os.Unsetenv("WECHAT_MCH_ID")
	os.Unsetenv("WECHAT_API_KEY")

	order, err := CreateOrder("user1", "pro", "monthly", 499, "https://example.com/notify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.OrderID == "" {
		t.Fatal("expected non-empty order ID")
	}
	if order.PaymentURL == "" {
		t.Fatal("expected non-empty payment URL")
	}
	if order.Amount != 499 {
		t.Fatalf("expected amount 499, got %d", order.Amount)
	}
	if order.Currency != "CNY" {
		t.Fatalf("expected CNY, got %s", order.Currency)
	}
}

func TestHandlePaymentNotify_Mock(t *testing.T) {
	record, err := HandlePaymentNotify([]byte("<xml></xml>"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record == nil {
		t.Fatal("expected non-nil record")
	}
	if record.Status != "paid" {
		t.Fatalf("expected paid, got %s", record.Status)
	}
	if record.Provider != "wechat" {
		t.Fatalf("expected wechat, got %s", record.Provider)
	}
}

func TestMockConfirmPayment(t *testing.T) {
	record := MockConfirmPayment("order_123")
	if record.ProviderOrderID != "order_123" {
		t.Fatalf("expected order_123, got %s", record.ProviderOrderID)
	}
	if record.Status != "paid" {
		t.Fatalf("expected paid, got %s", record.Status)
	}
}
