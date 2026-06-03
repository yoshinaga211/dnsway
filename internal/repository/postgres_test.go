package repository

import (
	"testing"
)

func testRepo(t *testing.T) *PostgresRepository {
	t.Helper()
	dir := t.TempDir()
	store := NewFileStore(dir)
	r := &PostgresRepository{
		isMock: true,
		store:  store,
		mock: &mockData{
			categories:    store.Data.Categories,
			allowlist:     store.Data.Allowlist,
			denylist:      store.Data.Denylist,
			timeWindows:   store.Data.TimeWindows,
			logs:          []map[string]interface{}{},
			profiles:      store.Data.Profiles,
			users:         store.Data.Users,
			subscriptions: store.Data.Subscriptions,
			payments:      store.Data.Payments,
			plans:         store.Data.Plans,
		},
	}
	r.seedDefaults()
	r.SeedPlans()
	return r
}

// ================================================================
// User Tests
// ================================================================

func TestCreateUser(t *testing.T) {
	r := testRepo(t)
	id, err := r.CreateUser("test@example.com", "hash123")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty user id")
	}
}

func TestGetUserByEmail(t *testing.T) {
	r := testRepo(t)
	r.CreateUser("find@example.com", "hash456")

	u, err := r.GetUserByEmail("find@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if u["email"] != "find@example.com" {
		t.Errorf("expected find@example.com, got %v", u["email"])
	}
	if u["password_hash"] != "hash456" {
		t.Errorf("expected hash456, got %v", u["password_hash"])
	}
}

func TestGetUserByEmailNotFound(t *testing.T) {
	r := testRepo(t)
	_, err := r.GetUserByEmail("nonexistent@example.com")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestGetUserByID(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("byid@example.com", "hash")

	u, err := r.GetUserByID(id)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if u["email"] != "byid@example.com" {
		t.Errorf("expected byid@example.com, got %v", u["email"])
	}
}

func TestGetUserByIDNotFound(t *testing.T) {
	r := testRepo(t)
	_, err := r.GetUserByID("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for non-existent user id")
	}
}

func TestGetUserByWechatUnionID(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("wechat@example.com", "hash")
	r.UpdateUserWechat(id, "union-123", "open-456", "wx_user", "https://example.com/avatar.png")

	u, err := r.GetUserByWechatUnionID("union-123")
	if err != nil {
		t.Fatalf("GetUserByWechatUnionID failed: %v", err)
	}
	if u["email"] != "wechat@example.com" {
		t.Errorf("expected wechat@example.com, got %v", u["email"])
	}
	if u["wechat_unionid"] != "union-123" {
		t.Errorf("expected union-123, got %v", u["wechat_unionid"])
	}
}

func TestGetUserByWechatUnionIDNotFound(t *testing.T) {
	r := testRepo(t)
	_, err := r.GetUserByWechatUnionID("nonexistent-union")
	if err == nil {
		t.Fatal("expected error for non-existent union id")
	}
}

func TestUpdateUserWechat(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("updatewx@example.com", "hash")

	err := r.UpdateUserWechat(id, "union-upd", "open-upd", "nick", "https://avatar.url")
	if err != nil {
		t.Fatalf("UpdateUserWechat failed: %v", err)
	}

	u, _ := r.GetUserByID(id)
	if u["wechat_unionid"] != "union-upd" {
		t.Errorf("expected union-upd, got %v", u["wechat_unionid"])
	}
	if u["wechat_openid"] != "open-upd" {
		t.Errorf("expected open-upd, got %v", u["wechat_openid"])
	}
	if u["wechat_nickname"] != "nick" {
		t.Errorf("expected nick, got %v", u["wechat_nickname"])
	}
}

func TestUpdateUserWechatUserNotFound(t *testing.T) {
	r := testRepo(t)
	err := r.UpdateUserWechat("nonexistent", "u", "o", "n", "a")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

// ================================================================
// Profile Tests
// ================================================================

func TestGetProfileConfig(t *testing.T) {
	r := testRepo(t)
	p, err := r.GetProfileConfig("test")
	if err != nil {
		t.Fatalf("GetProfileConfig failed: %v", err)
	}
	if p["profile_id"] != "test" {
		t.Errorf("expected test, got %v", p["profile_id"])
	}
	if p["safe_search"] != true {
		t.Errorf("expected safe_search true, got %v", p["safe_search"])
	}
	blocked, ok := p["blocked_categories"].([]string)
	if !ok {
		t.Fatal("expected blocked_categories as []string")
	}
	if len(blocked) != 2 {
		t.Errorf("expected 2 blocked categories, got %d", len(blocked))
	}
}

func TestGetProfileConfigUnknownProfile(t *testing.T) {
	r := testRepo(t)
	p, err := r.GetProfileConfig("nonexistent")
	if err != nil {
		t.Fatalf("GetProfileConfig for unknown should fallback to test profile: %v", err)
	}
	if p == nil {
		t.Fatal("expected fallback profile, got nil")
	}
}

func TestUpdateProfileSettings(t *testing.T) {
	r := testRepo(t)

	err := r.UpdateProfileSettings("test", boolPtr(false), boolPtr(true), boolPtr(true), intPtr(30), boolPtr(true), boolPtr(true), boolPtr(true), boolPtr(true), strPtr("eu"))
	if err != nil {
		t.Fatalf("UpdateProfileSettings failed: %v", err)
	}

	p, _ := r.GetProfileConfig("test")
	if p["safe_search"] != false {
		t.Errorf("expected safe_search false, got %v", p["safe_search"])
	}
	if p["bypass_prevention"] != true {
		t.Errorf("expected bypass_prevention true, got %v", p["bypass_prevention"])
	}
	if p["ai_threat_detection"] != true {
		t.Errorf("expected ai_threat_detection true, got %v", p["ai_threat_detection"])
	}
	if p["storage_region"] != "eu" {
		t.Errorf("expected storage_region eu, got %v", p["storage_region"])
	}
}

func TestUpdateProfileSettingsNewProfile(t *testing.T) {
	r := testRepo(t)

	err := r.UpdateProfileSettings("new-profile", boolPtr(true), nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateProfileSettings for new profile failed: %v", err)
	}

	p, _ := r.GetProfileConfig("new-profile")
	if p["safe_search"] != true {
		t.Errorf("expected safe_search true for new profile, got %v", p["safe_search"])
	}
}

// ================================================================
// Category Tests
// ================================================================

func TestGetCategoryConfigs(t *testing.T) {
	r := testRepo(t)
	cfg, err := r.GetCategoryConfigs("test")
	if err != nil {
		t.Fatalf("GetCategoryConfigs failed: %v", err)
	}
	if !cfg["CAT_001"] {
		t.Error("expected CAT_001 to be blocked")
	}
	if !cfg["CAT_008"] {
		t.Error("expected CAT_008 to be blocked")
	}
}

func TestGetCategoryConfigsEmpty(t *testing.T) {
	r := testRepo(t)
	cfg, err := r.GetCategoryConfigs("nonexistent")
	if err != nil {
		t.Fatalf("GetCategoryConfigs for unknown profile failed: %v", err)
	}
	if len(cfg) != 0 {
		t.Errorf("expected empty config, got %v", cfg)
	}
}

func TestUpdateCategoryConfig(t *testing.T) {
	r := testRepo(t)

	err := r.UpdateCategoryConfig("test", "CAT_005", true)
	if err != nil {
		t.Fatalf("UpdateCategoryConfig failed: %v", err)
	}

	cfg, _ := r.GetCategoryConfigs("test")
	if !cfg["CAT_005"] {
		t.Error("expected CAT_005 to be blocked after update")
	}
}

func TestUpdateCategoryConfigNewProfile(t *testing.T) {
	r := testRepo(t)

	err := r.UpdateCategoryConfig("new-profile", "CAT_002", true)
	if err != nil {
		t.Fatalf("UpdateCategoryConfig for new profile failed: %v", err)
	}

	cfg, _ := r.GetCategoryConfigs("new-profile")
	if !cfg["CAT_002"] {
		t.Error("expected CAT_002 to be blocked for new profile")
	}
}

// ================================================================
// Allowlist Tests
// ================================================================

func TestGetAllowlist(t *testing.T) {
	r := testRepo(t)
	list, err := r.GetAllowlist("test")
	if err != nil {
		t.Fatalf("GetAllowlist failed: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 entries, got %d", len(list))
	}
}

func TestGetAllowlistEmpty(t *testing.T) {
	r := testRepo(t)
	list, err := r.GetAllowlist("nonexistent")
	if err != nil {
		t.Fatalf("GetAllowlist for unknown profile failed: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestAddToAllowlist(t *testing.T) {
	r := testRepo(t)
	err := r.AddToAllowlist("test", "newsite.com")
	if err != nil {
		t.Fatalf("AddToAllowlist failed: %v", err)
	}

	list, _ := r.GetAllowlist("test")
	found := false
	for _, d := range list {
		if d == "newsite.com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected newsite.com in allowlist")
	}
}

func TestAddToAllowlistNewProfile(t *testing.T) {
	r := testRepo(t)
	err := r.AddToAllowlist("new", "domain.com")
	if err != nil {
		t.Fatalf("AddToAllowlist failed: %v", err)
	}
	list, _ := r.GetAllowlist("new")
	if len(list) != 1 || list[0] != "domain.com" {
		t.Errorf("expected [domain.com], got %v", list)
	}
}

func TestRemoveFromAllowlist(t *testing.T) {
	r := testRepo(t)
	err := r.RemoveFromAllowlist("test", "khanacademy.org")
	if err != nil {
		t.Fatalf("RemoveFromAllowlist failed: %v", err)
	}

	list, _ := r.GetAllowlist("test")
	for _, d := range list {
		if d == "khanacademy.org" {
			t.Error("khanacademy.org should have been removed")
		}
	}
}

func TestRemoveFromAllowlistNonexistent(t *testing.T) {
	r := testRepo(t)
	err := r.RemoveFromAllowlist("test", "nonexistent.com")
	if err != nil {
		t.Errorf("removing non-existent domain should not error: %v", err)
	}
}

// ================================================================
// Denylist Tests
// ================================================================

func TestGetDenylist(t *testing.T) {
	r := testRepo(t)
	list, err := r.GetDenylist("test")
	if err != nil {
		t.Fatalf("GetDenylist failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 entries, got %d", len(list))
	}
}

func TestGetDenylistEmpty(t *testing.T) {
	r := testRepo(t)
	list, err := r.GetDenylist("nonexistent")
	if err != nil {
		t.Fatalf("GetDenylist failed: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestAddToDenylist(t *testing.T) {
	r := testRepo(t)
	err := r.AddToDenylist("test", "badsite.com", "Test block")
	if err != nil {
		t.Fatalf("AddToDenylist failed: %v", err)
	}

	list, _ := r.GetDenylist("test")
	for _, entry := range list {
		if entry["domain"] == "badsite.com" {
			if entry["reason"] != "Test block" {
				t.Errorf("expected reason 'Test block', got %v", entry["reason"])
			}
			return
		}
	}
	t.Error("expected badsite.com in denylist")
}

func TestRemoveFromDenylist(t *testing.T) {
	r := testRepo(t)
	err := r.RemoveFromDenylist("test", "malicious-site.net")
	if err != nil {
		t.Fatalf("RemoveFromDenylist failed: %v", err)
	}

	list, _ := r.GetDenylist("test")
	for _, entry := range list {
		if entry["domain"] == "malicious-site.net" {
			t.Error("malicious-site.net should have been removed")
		}
	}
}

func TestGetDenylistDomains(t *testing.T) {
	r := testRepo(t)
	domains, err := r.GetDenylistDomains("test")
	if err != nil {
		t.Fatalf("GetDenylistDomains failed: %v", err)
	}
	if len(domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(domains))
	}
}

func TestGetDenylistDomainsEmpty(t *testing.T) {
	r := testRepo(t)
	domains, err := r.GetDenylistDomains("nonexistent")
	if err != nil {
		t.Fatalf("GetDenylistDomains failed: %v", err)
	}
	if len(domains) != 0 {
		t.Errorf("expected 0 domains, got %d", len(domains))
	}
}

// ================================================================
// Time Window Tests
// ================================================================

func TestGetTimeWindows(t *testing.T) {
	r := testRepo(t)
	list, err := r.GetTimeWindows("test")
	if err != nil {
		t.Fatalf("GetTimeWindows failed: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected at least 1 time window")
	}
	if list[0]["profile_id"] != "test" {
		t.Errorf("expected profile_id test, got %v", list[0]["profile_id"])
	}
}

func TestGetTimeWindowsEmpty(t *testing.T) {
	r := testRepo(t)
	list, err := r.GetTimeWindows("nonexistent")
	if err != nil {
		t.Fatalf("GetTimeWindows failed: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestAddTimeWindow(t *testing.T) {
	r := testRepo(t)
	tw := map[string]interface{}{
		"name":         "Weekday Gaming",
		"days_of_week": []int{1, 2, 3, 4, 5},
		"start_time":   "18:00",
		"end_time":     "20:00",
		"timezone":     "Asia/Shanghai",
		"target_type":  "CATEGORY",
		"target_value": "CAT_005",
	}
	err := r.AddTimeWindow("test", tw)
	if err != nil {
		t.Fatalf("AddTimeWindow failed: %v", err)
	}

	list, _ := r.GetTimeWindows("test")
	if len(list) != 2 {
		t.Errorf("expected 2 time windows, got %d", len(list))
	}
}

func TestDeleteTimeWindow(t *testing.T) {
	r := testRepo(t)
	err := r.DeleteTimeWindow("test", "tw-default")
	if err != nil {
		t.Fatalf("DeleteTimeWindow failed: %v", err)
	}

	list, _ := r.GetTimeWindows("test")
	if len(list) != 0 {
		t.Errorf("expected 0 time windows after delete, got %d", len(list))
	}
}

// ================================================================
// Query Log Tests
// ================================================================

func TestLogQuery(t *testing.T) {
	r := testRepo(t)
	err := r.LogQuery("test", "example.com", 0, "P6: Default", []string{}, 100, "")
	if err != nil {
		t.Fatalf("LogQuery failed: %v", err)
	}
}

func TestGetQueryLogs(t *testing.T) {
	r := testRepo(t)
	r.LogQuery("test", "allowed.com", 0, "P6: Allow", []string{}, 50, "")
	r.LogQuery("test", "blocked.com", 1, "P2: Deny", []string{"CAT_001"}, 200, "")

	logs, err := r.GetQueryLogs("test", 10)
	if err != nil {
		t.Fatalf("GetQueryLogs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}
	// Most recent first
	if logs[0]["domain"] != "blocked.com" {
		t.Errorf("expected blocked.com first, got %v", logs[0]["domain"])
	}
	if logs[0]["decision"] != "BLOCK" {
		t.Errorf("expected BLOCK, got %v", logs[0]["decision"])
	}
	if logs[1]["decision"] != "ALLOW" {
		t.Errorf("expected ALLOW, got %v", logs[1]["decision"])
	}
}

func TestGetQueryLogsEmpty(t *testing.T) {
	r := testRepo(t)
	logs, err := r.GetQueryLogs("nonexistent", 10)
	if err != nil {
		t.Fatalf("GetQueryLogs failed: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected empty logs, got %d", len(logs))
	}
}

func TestGetQueryLogsLimit(t *testing.T) {
	r := testRepo(t)
	for i := 0; i < 10; i++ {
		r.LogQuery("test", "domain.com", 0, "test", []string{}, 0, "")
	}

	logs, _ := r.GetQueryLogs("test", 3)
	if len(logs) != 3 {
		t.Errorf("expected 3 logs, got %d", len(logs))
	}
}

func TestLogQueryMax200(t *testing.T) {
	r := testRepo(t)
	for i := 0; i < 250; i++ {
		r.LogQuery("test", "domain.com", 0, "test", []string{}, 0, "")
	}

	logs, _ := r.GetQueryLogs("test", 250)
	if len(logs) > 200 {
		t.Errorf("logs should be capped at 200, got %d", len(logs))
	}
}

// ================================================================
// Plan Tests
// ================================================================

func TestGetPlans(t *testing.T) {
	r := testRepo(t)
	plans, err := r.GetPlans()
	if err != nil {
		t.Fatalf("GetPlans failed: %v", err)
	}
	if len(plans) != 3 {
		t.Errorf("expected 3 plans, got %d", len(plans))
	}
}

func TestGetPlan(t *testing.T) {
	r := testRepo(t)
	p, err := r.GetPlan("pro")
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if p["name"] != "Pro" {
		t.Errorf("expected Pro, got %v", p["name"])
	}
	if p["price_monthly"] != 2900 {
		t.Errorf("expected 2900, got %v", p["price_monthly"])
	}
}

func TestGetPlanNotFound(t *testing.T) {
	r := testRepo(t)
	_, err := r.GetPlan("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent plan")
	}
}

// ================================================================
// Subscription Tests
// ================================================================

func TestCreateSubscription(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("sub@example.com", "hash")
	subID, err := r.CreateSubscription(id, "pro", 7)
	if err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}
	if subID == "" {
		t.Fatal("expected non-empty subscription id")
	}
}

func TestGetSubscription(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("subget@example.com", "hash")
	r.CreateSubscription(id, "free", 7)

	sub, err := r.GetSubscription(id)
	if err != nil {
		t.Fatalf("GetSubscription failed: %v", err)
	}
	if sub["plan_id"] != "free" {
		t.Errorf("expected free, got %v", sub["plan_id"])
	}
	if sub["status"] != "trialing" {
		t.Errorf("expected trialing, got %v", sub["status"])
	}
}

func TestGetSubscriptionNotFound(t *testing.T) {
	r := testRepo(t)
	_, err := r.GetSubscription("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent subscription")
	}
}

func TestActivateSubscription(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("act@example.com", "hash")
	r.CreateSubscription(id, "free", 7)

	err := r.ActivateSubscription(id, "pro", "monthly")
	if err != nil {
		t.Fatalf("ActivateSubscription failed: %v", err)
	}

	sub, _ := r.GetSubscription(id)
	if sub["status"] != "active" {
		t.Errorf("expected active, got %v", sub["status"])
	}
	if sub["plan_id"] != "pro" {
		t.Errorf("expected pro, got %v", sub["plan_id"])
	}
}

func TestActivateSubscriptionNotFound(t *testing.T) {
	r := testRepo(t)
	err := r.ActivateSubscription("nonexistent", "pro", "monthly")
	if err == nil {
		t.Fatal("expected error for non-existent subscription")
	}
}

func TestActivateSubscriptionYearly(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("actyear@example.com", "hash")
	r.CreateSubscription(id, "free", 7)

	err := r.ActivateSubscription(id, "family", "yearly")
	if err != nil {
		t.Fatalf("ActivateSubscription yearly failed: %v", err)
	}

	sub, _ := r.GetSubscription(id)
	if sub["plan_id"] != "family" {
		t.Errorf("expected family, got %v", sub["plan_id"])
	}
}

func TestUpdateSubscriptionStripe(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("stripe@example.com", "hash")
	r.CreateSubscription(id, "free", 7)

	err := r.UpdateSubscriptionStripe(id, "pro", "active", "sub_stripe_123", "2026-06-01T00:00:00Z")
	if err != nil {
		t.Fatalf("UpdateSubscriptionStripe failed: %v", err)
	}

	sub, _ := r.GetSubscription(id)
	if sub["status"] != "active" {
		t.Errorf("expected active, got %v", sub["status"])
	}
}

func TestCancelSubscriptionStripe(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("cancel@example.com", "hash")
	r.CreateSubscription(id, "pro", 7)

	err := r.CancelSubscriptionStripe(id)
	if err != nil {
		t.Fatalf("CancelSubscriptionStripe failed: %v", err)
	}

	sub, _ := r.GetSubscription(id)
	if sub["status"] != "canceled" {
		t.Errorf("expected canceled, got %v", sub["status"])
	}
}

func TestCancelSubscriptionStripeNotFound(t *testing.T) {
	r := testRepo(t)
	err := r.CancelSubscriptionStripe("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent subscription")
	}
}

// ================================================================
// Payment Record Tests
// ================================================================

func TestCreatePaymentRecord(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("pay@example.com", "hash")
	err := r.CreatePaymentRecord(id, "pro", "wechat", "order-123", 2900)
	if err != nil {
		t.Fatalf("CreatePaymentRecord failed: %v", err)
	}
}

func TestConfirmPayment(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("confirm@example.com", "hash")
	r.CreatePaymentRecord(id, "pro", "wechat", "order-456", 2900)

	err := r.ConfirmPayment("order-456")
	if err != nil {
		t.Fatalf("ConfirmPayment failed: %v", err)
	}

	p, err := r.GetPaymentByOrderID("order-456")
	if err != nil {
		t.Fatalf("GetPaymentByOrderID failed: %v", err)
	}
	if p["status"] != "paid" {
		t.Errorf("expected paid, got %v", p["status"])
	}
}

func TestConfirmPaymentNotFound(t *testing.T) {
	r := testRepo(t)
	err := r.ConfirmPayment("nonexistent-order")
	if err == nil {
		t.Fatal("expected error for non-existent payment")
	}
}

func TestGetPaymentByOrderID(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("getpay@example.com", "hash")
	r.CreatePaymentRecord(id, "free", "wechat", "order-789", 0)

	p, err := r.GetPaymentByOrderID("order-789")
	if err != nil {
		t.Fatalf("GetPaymentByOrderID failed: %v", err)
	}
	if p["user_id"] != id {
		t.Errorf("expected user_id %s, got %v", id, p["user_id"])
	}
	if p["amount"] != 0 {
		t.Errorf("expected 0, got %v", p["amount"])
	}
}

func TestGetPaymentByOrderIDNotFound(t *testing.T) {
	r := testRepo(t)
	_, err := r.GetPaymentByOrderID("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent payment order")
	}
}

// ================================================================
// Stripe User Tests
// ================================================================

func TestUpdateUserStripeCustomer(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("stripeuser@example.com", "hash")

	err := r.UpdateUserStripeCustomer(id, "cus_stripe_123")
	if err != nil {
		t.Fatalf("UpdateUserStripeCustomer failed: %v", err)
	}

	u, _ := r.GetUserByID(id)
	if u["stripe_customer_id"] != "cus_stripe_123" {
		t.Errorf("expected cus_stripe_123, got %v", u["stripe_customer_id"])
	}
}

func TestUpdateUserStripeCustomerUserNotFound(t *testing.T) {
	r := testRepo(t)
	err := r.UpdateUserStripeCustomer("nonexistent", "cus_123")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestGetUserByStripeCustomerID(t *testing.T) {
	r := testRepo(t)
	id, _ := r.CreateUser("stripeget@example.com", "hash")
	r.UpdateUserStripeCustomer(id, "cus_find_me")

	u, err := r.GetUserByStripeCustomerID("cus_find_me")
	if err != nil {
		t.Fatalf("GetUserByStripeCustomerID failed: %v", err)
	}
	if u["email"] != "stripeget@example.com" {
		t.Errorf("expected stripeget@example.com, got %v", u["email"])
	}
}

func TestGetUserByStripeCustomerIDNotFound(t *testing.T) {
	r := testRepo(t)
	_, err := r.GetUserByStripeCustomerID("cus_nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent stripe customer")
	}
}

// ================================================================
// Profile Listing Tests
// ================================================================

func TestListProfiles(t *testing.T) {
	r := testRepo(t)
	profiles, err := r.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles failed: %v", err)
	}
	if len(profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestListProfilesContent(t *testing.T) {
	r := testRepo(t)
	profiles, _ := r.ListProfiles()

	foundTest := false
	for _, p := range profiles {
		if p["profile_id"] == "test" && p["name"] == "Test Profile" {
			foundTest = true
			break
		}
	}
	if !foundTest {
		t.Error("expected test profile in list")
	}
}

// ================================================================
// Edge Cases
// ================================================================

func TestUpdateProfileSettingsAllNil(t *testing.T) {
	r := testRepo(t)
	err := r.UpdateProfileSettings("test", nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateProfileSettings with all nil should not error: %v", err)
	}
}

func TestMultipleUsers(t *testing.T) {
	r := testRepo(t)
	emails := []string{"a@test.com", "b@test.com", "c@test.com"}
	for _, e := range emails {
		r.CreateUser(e, "hash")
	}

	for _, e := range emails {
		u, err := r.GetUserByEmail(e)
		if err != nil {
			t.Errorf("GetUserByEmail(%s) failed: %v", e, err)
		}
		if u["email"] != e {
			t.Errorf("expected %s, got %v", e, u["email"])
		}
	}
}

func TestGetQueryLogsReverseOrder(t *testing.T) {
	r := testRepo(t)
	domains := []string{"first.com", "second.com", "third.com"}
	for _, d := range domains {
		r.LogQuery("test", d, 0, "test", []string{}, 0, "")
	}

	logs, _ := r.GetQueryLogs("test", 10)
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}
	if logs[0]["domain"] != "third.com" {
		t.Errorf("expected third.com first (reverse order), got %v", logs[0]["domain"])
	}
	if logs[2]["domain"] != "first.com" {
		t.Errorf("expected first.com last, got %v", logs[2]["domain"])
	}
}

// ================================================================
// Helpers
// ================================================================

func boolPtr(b bool) *bool    { return &b }
func intPtr(i int) *int       { return &i }
func strPtr(s string) *string { return &s }
