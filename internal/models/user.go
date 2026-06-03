package models

// ── User ──

type User struct {
	ID                string `json:"id"`
	Email             string `json:"email"`
	PasswordHash      string `json:"-"`
	WechatUnionID     string `json:"wechat_union_id,omitempty"`
	WechatOpenID      string `json:"wechat_open_id,omitempty"`
	WechatNickname    string `json:"wechat_nickname,omitempty"`
	WechatAvatar      string `json:"wechat_avatar,omitempty"`
	StripeCustomerID  string `json:"stripe_customer_id,omitempty"`
	Status            string `json:"status"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

// ── Plan ──

type Plan struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	PriceMonthly int      `json:"price_monthly"` // cents (CNY fen)
	PriceYearly  int      `json:"price_yearly"`
	Features     []string `json:"features"`
	IsActive     bool     `json:"is_active"`
}

// ── Subscription ──

type Subscription struct {
	ID                 string `json:"id"`
	UserID             string `json:"user_id"`
	PlanID             string `json:"plan_id"`
	Status             string `json:"status"` // trialing, active, past_due, canceled, expired
	TrialEndsAt        string `json:"trial_ends_at,omitempty"`
	CurrentPeriodStart string `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   string `json:"current_period_end,omitempty"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// ── PaymentRecord ──

type PaymentRecord struct {
	ID              string `json:"id"`
	UserID          string `json:"user_id"`
	PlanID          string `json:"plan_id"`
	Amount          int    `json:"amount"` // cents
	Currency        string `json:"currency"`
	Provider        string `json:"provider"` // wechat, email_invoice
	ProviderOrderID string `json:"provider_order_id,omitempty"`
	Status          string `json:"status"` // pending, paid, failed, refunded
	PaidAt          string `json:"paid_at,omitempty"`
	CreatedAt       string `json:"created_at"`
}

// ── JWT ──

type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Exp    int64  `json:"exp"`
	Iat    int64  `json:"iat"`
}

// ── Request / Response ──

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreateOrderRequest struct {
	PlanID string `json:"plan_id"`
	Period string `json:"period"` // monthly, yearly
}

type AuthResponse struct {
	Token          string        `json:"token"`
	User           *User         `json:"user"`
	Subscription   *Subscription `json:"subscription"`
	TrialRemaining int           `json:"trial_remaining_days"`
}

type WechatQRResponse struct {
	QRURL string `json:"qr_url"`
	State string `json:"state"`
}

type PlanResponse struct {
	Plans []Plan `json:"plans"`
}

type OrderResponse struct {
	OrderID     string `json:"order_id"`
	PaymentURL  string `json:"payment_url,omitempty"`
	QRCode      string `json:"qr_code,omitempty"`
	Amount      int    `json:"amount"`
	Currency    string `json:"currency"`
	Description string `json:"description"`
}

// ── WeChat OAuth ──

type WechatOAuthToken struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	UnionID      string `json:"unionid"`
	Scope        string `json:"scope"`
}

type WechatUserInfo struct {
	OpenID     string `json:"openid"`
	UnionID    string `json:"unionid"`
	Nickname   string `json:"nickname"`
	HeadImgURL string `json:"headimgurl"`
	Sex        int    `json:"sex"`
	Province   string `json:"province"`
	City       string `json:"city"`
	Country    string `json:"country"`
}

// ── WeChat Pay ──

type WechatPayOrder struct {
	AppID      string `json:"appid"`
	MchID      string `json:"mchid"`
	OutTradeNo string `json:"out_trade_no"`
	TotalFee   int    `json:"total_fee"`
	Body       string `json:"body"`
	OpenID     string `json:"openid"`
	NotifyURL  string `json:"notify_url"`
	TradeType  string `json:"trade_type"` // JSAPI, NATIVE
}

// ── Stripe ──

type StripePlan struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	PriceMonthly int    `json:"price_monthly"` // cents (USD)
	PriceYearly  int    `json:"price_yearly"`  // cents (USD)
	PriceIDMonthly string `json:"price_id_monthly,omitempty"`
	PriceIDYearly  string `json:"price_id_yearly,omitempty"`
	Features     []string `json:"features"`
	IsActive     bool   `json:"is_active"`
}

type CheckoutSessionRequest struct {
	PlanID string `json:"plan_id"`
	Period string `json:"period"` // monthly, yearly
}

type CheckoutSessionResponse struct {
	SessionID string `json:"session_id"`
	URL       string `json:"url"`
}

type SubscriptionStatusResponse struct {
	Status           string `json:"status"`
	Plan             string `json:"plan"`
	PlanName         string `json:"plan_name"`
	CurrentPeriodEnd string `json:"current_period_end"`
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
	TrialRemaining   int    `json:"trial_remaining_days"`
	IsTrialing       bool   `json:"is_trialing"`
}

type StripeWebhookEvent struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}
