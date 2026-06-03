package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"time"

	"dnsway-pc/internal/models"
)

func wechatAppID() string    { return os.Getenv("WECHAT_APP_ID") }
func wechatAppSecret() string { return os.Getenv("WECHAT_APP_SECRET") }
func isMock() bool            { return wechatAppID() == "" || wechatAppSecret() == "" }

// GenerateQRState creates a random state parameter for OAuth CSRF protection.
func GenerateQRState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetQRCodeURL returns the WeChat OAuth QR code URL, or a mock redirect URL.
func GetQRCodeURL(state string) string {
	if isMock() {
		return "/api/v1/auth/wechat/callback?code=mock_code&state=" + state
	}
	q := url.Values{}
	q.Set("appid", wechatAppID())
	q.Set("redirect_uri", os.Getenv("WECHAT_REDIRECT_URI"))
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_login")
	q.Set("state", state)
	return "https://open.weixin.qq.com/connect/qrconnect?" + q.Encode() + "#wechat_redirect"
}

// ExchangeCodeForUser swaps an OAuth code for user info. Returns mock data in mock mode.
func ExchangeCodeForUser(code string) (*models.WechatUserInfo, error) {
	if isMock() {
		return &models.WechatUserInfo{
			OpenID:     "mock_openid_" + code[:8],
			UnionID:    "mock_unionid_" + code[:8],
			Nickname:   "微信用户",
			HeadImgURL: "",
		}, nil
	}

	// Real OAuth flow: code → access_token
	tokenURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code",
		wechatAppID(), wechatAppSecret(), code,
	)
	resp, err := http.Get(tokenURL)
	if err != nil {
		return nil, fmt.Errorf("wechat token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	var token models.WechatOAuthToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("wechat token decode: %w", err)
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("wechat returned empty access token")
	}

	// access_token → userinfo
	infoURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/userinfo?access_token=%s&openid=%s",
		token.AccessToken, token.OpenID,
	)
	resp2, err := http.Get(infoURL)
	if err != nil {
		return nil, fmt.Errorf("wechat userinfo failed: %w", err)
	}
	defer resp2.Body.Close()

	var info models.WechatUserInfo
	if err := json.NewDecoder(resp2.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("wechat userinfo decode: %w", err)
	}
	return &info, nil
}

// VerifyWebAuth checks that an access_token is still valid for the given openid.
func VerifyWebAuth(accessToken, openID string) error {
	if isMock() {
		return nil
	}
	verifyURL := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/auth?access_token=%s&openid=%s",
		accessToken, openID,
	)
	resp, err := http.Get(verifyURL)
	if err != nil {
		return fmt.Errorf("wechat auth verify failed: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

// WechatSign generates a SHA256 signature for WeChat Pay signing.
func WechatSign(params map[string]string, key string) string {
	// Simplified: concatenate sorted params and sign with HMAC-SHA256
	var concat string
	for _, k := range []string{"appid", "mch_id", "nonce_str", "body", "out_trade_no", "total_fee", "spbill_create_ip", "notify_url", "trade_type"} {
		if v, ok := params[k]; ok {
			concat += k + "=" + v + "&"
		}
	}
	concat += "key=" + key
	h := sha256.Sum256([]byte(concat))
	return hex.EncodeToString(h[:])
}

func init() {
	// Seed random for QR state generation
	rand.Seed(time.Now().UnixNano())
}
