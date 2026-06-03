package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"dnsway-pc/internal/models"

	"github.com/gin-gonic/gin"
)

const claimsKey = "auth_claims"

func jwtSecret() []byte {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		s = "dns1-dev-secret-change-in-production"
	}
	return []byte(s)
}

// GenerateToken creates a signed JWT for the given user.
func GenerateToken(userID, email string) (string, error) {
	now := time.Now()
	claims := models.JWTClaims{
		UserID: userID,
		Email:  email,
		Exp:    now.Add(168 * time.Hour).Unix(),
		Iat:    now.Unix(),
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sigPart := fmt.Sprintf("%s.%s", header, payload)
	mac := hmac.New(sha256.New, jwtSecret())
	mac.Write([]byte(sigPart))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", sigPart, sig), nil
}

// ValidateToken parses and validates a JWT, returning claims on success.
func ValidateToken(tokenStr string) (*models.JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}
	sigPart := fmt.Sprintf("%s.%s", parts[0], parts[1])
	mac := hmac.New(sha256.New, jwtSecret())
	mac.Write([]byte(sigPart))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, errors.New("invalid token signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid token payload")
	}
	var claims models.JWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, errors.New("invalid token claims")
	}
	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
	}
	return &claims, nil
}

// GetClaimsFromContext extracts JWT claims set by AuthMiddleware.
func GetClaimsFromContext(c *gin.Context) *models.JWTClaims {
	if v, ok := c.Get(claimsKey); ok {
		if claims, ok := v.(*models.JWTClaims); ok {
			return claims
		}
	}
	return nil
}

// SetClaimsInContext stores claims in the Gin context.
func SetClaimsInContext(c *gin.Context, claims *models.JWTClaims) {
	c.Set(claimsKey, claims)
}
