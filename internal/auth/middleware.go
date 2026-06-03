package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates the JWT from the Authorization header and sets claims in context.
// Returns 401 if the token is missing or invalid.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "details": "Missing or invalid Authorization header"})
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := ValidateToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "details": err.Error()})
			return
		}
		SetClaimsInContext(c, claims)
		c.Next()
	}
}

// OptionalAuthMiddleware validates the JWT if present, but does not reject missing tokens.
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header != "" && strings.HasPrefix(header, "Bearer ") {
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			if claims, err := ValidateToken(tokenStr); err == nil {
				SetClaimsInContext(c, claims)
			}
		}
		c.Next()
	}
}
