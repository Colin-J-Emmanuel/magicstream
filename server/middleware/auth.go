package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Colin-J-Emmanuel/magicstream/server/auth"
	"github.com/Colin-J-Emmanuel/magicstream/server/models"
)

const (
	ctxUserID = "userID"
	ctxRole   = "role"
)

// RequireAuth rejects requests without a valid access token and records the
// caller's identity on the context for downstream handlers.
func RequireAuth(tokens *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := c.Cookie("access_token")
		if err != nil || raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		claims, err := tokens.ParseAccess(raw)
		if err != nil {
			log.Printf("rejected access token: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		c.Set(ctxUserID, claims.Subject)
		c.Set(ctxRole, claims.Role)
		c.Next()
	}
}

// UserID returns the authenticated user's ID. Only valid behind RequireAuth.
func UserID(c *gin.Context) string { return c.GetString(ctxUserID) }

// Role returns the authenticated user's role. Only valid behind RequireAuth.
func Role(c *gin.Context) string { return c.GetString(ctxRole) }

// RequireRole must run after RequireAuth. It rejects authenticated callers without the role.
// If RequireAuth is missing, the role is empty and every request is rejected: it fails closed.
func RequireRole(role models.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		if Role(c) != string(role) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}
