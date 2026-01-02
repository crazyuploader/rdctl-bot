package web

import (
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
	return func(c *fiber.Ctx) error {
		providedKey := c.Get("X-API-Key")

		if providedKey == "" || subtle.ConstantTimeCompare([]byte(providedKey), []byte(apiKey)) != 1 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Unauthorized: Invalid or missing API Key",
			})
		}
		return c.Next()
	}
}

// DualAuth is a middleware that accepts either API key or token authentication
func DualAuth(apiKey string, tokenStore *TokenStore) fiber.Handler {
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
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Unauthorized: Invalid or expired token",
			})
		}

		// 2. Try API Key Authentication via Header (Constant Time)
		providedKey := c.Get("X-API-Key")
		if providedKey != "" && subtle.ConstantTimeCompare([]byte(providedKey), []byte(apiKey)) == 1 {
			// API key grants admin access
			c.Locals(ContextKeyAuthType, "api_key")
			c.Locals(ContextKeyRole, RoleAdmin)
			return c.Next()
		}

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized: Invalid or missing credentials",
		})
	}
}

// AdminOnly is a middleware that restricts access to admin users only
func AdminOnly(tokenStore *TokenStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals(ContextKeyRole).(Role)
		if !ok {
			// No role set, check if API key auth (which is always admin)
			authType, _ := c.Locals(ContextKeyAuthType).(string)
			if authType == "api_key" {
				return c.Next()
			}
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "Forbidden: Admin access required",
			})
		}

		if role != RoleAdmin {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "Forbidden: Admin access required for this operation",
			})
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
