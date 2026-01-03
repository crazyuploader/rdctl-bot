package web

import (
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Context keys for storing auth info
const (
	ContextKeyAuthType = "auth_type"  // "api_key" or "token"
	ContextKeyToken    = "auth_token" // *Token if authenticated via token
	ContextKeyRole     = "auth_role"  // Role if authenticated via token
)

// APIKeyAuth is a middleware for simple API key authentication (legacy, kept for compatibility)
func APIKeyAuth(apiKey string) fiber.Handler {
	apiKeyHash := sha256.Sum256([]byte(apiKey))
	return func(c *fiber.Ctx) error {
		providedKey := c.Get("X-API-Key")
		providedKeyHash := sha256.Sum256([]byte(providedKey))

		if subtle.ConstantTimeCompare(providedKeyHash[:], apiKeyHash[:]) != 1 {
			return fiber.NewError(fiber.StatusUnauthorized, "Unauthorized: Invalid or missing API Key")
		}
		return c.Next()
	}
}

// DualAuth is a middleware that accepts either API key or token authentication
func DualAuth(apiKey string, tokenStore *TokenStore, ipManager *IPManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Try Token Authentication via Headers
		tokenID := c.Get("X-Auth-Token")
		if tokenID == "" {
			// Try Authorization: Bearer <token>
			authHeader := c.Get("Authorization")
			if after, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
				tokenID = after
			}
		}

		if tokenID != "" && tokenStore != nil {
			// Validate token
			token, valid := tokenStore.ValidateToken(tokenID)
			if valid {
				// Store token info in context for later use
				c.Locals(ContextKeyAuthType, "token")
				c.Locals(ContextKeyToken, token)
				c.Locals(ContextKeyRole, token.Role)
				return c.Next()
			}
			// Token provided but invalid
			ipManager.RegisterAuthFailure(c.IP())
			return fiber.NewError(fiber.StatusUnauthorized, "Unauthorized: Invalid or expired token")
		}

		// 2. Try API Key Authentication via Header (Constant Time)
		providedKey := c.Get("X-API-Key")
		apiKeyHash := sha256.Sum256([]byte(apiKey))
		providedKeyHash := sha256.Sum256([]byte(providedKey))

		if subtle.ConstantTimeCompare(providedKeyHash[:], apiKeyHash[:]) == 1 {
			// API key grants admin access
			c.Locals(ContextKeyAuthType, "api_key")
			c.Locals(ContextKeyRole, RoleAdmin)
			return c.Next()
		}

		// Only register failure if BOTH methods fail and at least one was attempted/missing
		// Note: A missing header counts as a failure if we reach here
		ipManager.RegisterAuthFailure(c.IP())
		return fiber.NewError(fiber.StatusUnauthorized, "Unauthorized: Invalid or missing credentials")
	}
}

// AdminOnly is a middleware that restricts access to admin users only
func AdminOnly(tokenStore *TokenStore, ipManager *IPManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals(ContextKeyRole).(Role)
		if !ok {
			// No role set, check if API key auth (which is always admin)
			authType, _ := c.Locals(ContextKeyAuthType).(string)
			if authType == "api_key" {
				return c.Next()
			}
			// Should be caught by DualAuth first, but safe to log here too for deeper layers
			ipManager.RegisterAuthFailure(c.IP())
			return fiber.NewError(fiber.StatusForbidden, "Forbidden: Admin access required")
		}

		if role != RoleAdmin {
			// User is authenticated but not authorized for this resource.
			// This is NOT an auth failure (bad password) but a permission issue.
			// We DO NOT ban for permission issues normally, only for failed logins.
			return fiber.NewError(fiber.StatusForbidden, "Forbidden: Admin access required for this operation")
		}

		return c.Next()
	}
}

// GetRole returns the role from context
func GetRole(c *fiber.Ctx) Role {
	role, ok := c.Locals(ContextKeyRole).(Role)
	if !ok {
		return RoleViewer // Default to viewer if not set
	}
	return role
}

// GetToken returns the token from context
func GetToken(c *fiber.Ctx) *Token {
	token, _ := c.Locals(ContextKeyToken).(*Token)
	return token
}
