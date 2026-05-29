package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/vmOrbit/backend/internal/domain/port"
)

const (
	ContextKeyUserID   = "user_id"
	ContextKeyUsername = "username"
	ContextKeyRoles    = "roles"
	ContextKeyClaims   = "claims"
)

// contextUserIDKey is an unexported type for the user ID context key.
// Using a private type prevents collisions with other packages.
type contextUserIDKey struct{}

// UserIDFromContext extracts the authenticated user ID from a standard
// context.Context. Returns empty string if not set.
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextUserIDKey{}).(string)
	return v
}

// Auth validates the Bearer token and injects claims into the context.
func Auth(authSvc port.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			return
		}

		claims, err := authSvc.ValidateAccessToken(c.Request.Context(), parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyRoles, claims.Roles)
		c.Set(ContextKeyClaims, claims)

		// Also inject into the request context so service/repo layers can read
		// the user ID without depending on Gin.
		ctx := context.WithValue(c.Request.Context(), contextUserIDKey{}, claims.UserID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// RequirePermission checks that the authenticated user has the required resource/action permission.
// Permissions are embedded in the JWT as "resource:action" strings.
// Super-admins (role name "super_admin") bypass all checks.
func RequirePermission(resource, action string) gin.HandlerFunc {
	required := resource + ":" + action
	return func(c *gin.Context) {
		claims := GetCurrentClaims(c)
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		// Super-admin bypasses all permission checks
		for _, r := range claims.Roles {
			if r == "super_admin" {
				c.Next()
				return
			}
		}

		// Check JWT-embedded permissions ("resource:action")
		for _, p := range claims.Permissions {
			if p == required {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": fmt.Sprintf("permission denied: %s", required),
		})
	}
}

// WSAuth is like Auth but also accepts the JWT via the ?token= query parameter.
// Browsers cannot set custom headers on WebSocket connections, so the token
// must be passed as a query param for the initial HTTP upgrade request.
func WSAuth(authSvc port.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try Authorization header first, then ?token= query param.
		token := ""
		header := c.GetHeader("Authorization")
		if header != "" {
			parts := strings.SplitN(header, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
				token = parts[1]
			}
		}
		if token == "" {
			token = c.Query("token")
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		claims, err := authSvc.ValidateAccessToken(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyRoles, claims.Roles)
		c.Set(ContextKeyClaims, claims)

		ctx := context.WithValue(c.Request.Context(), contextUserIDKey{}, claims.UserID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// GetCurrentUserID extracts the authenticated user ID from the Gin context.
func GetCurrentUserID(c *gin.Context) string {
	id, _ := c.Get(ContextKeyUserID)
	s, _ := id.(string)
	return s
}

// GetCurrentClaims extracts the full claims from the Gin context.
func GetCurrentClaims(c *gin.Context) *port.Claims {
	v, _ := c.Get(ContextKeyClaims)
	claims, _ := v.(*port.Claims)
	return claims
}
