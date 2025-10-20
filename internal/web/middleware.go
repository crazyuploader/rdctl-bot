package web

import (
	"github.com/gofiber/fiber/v2"
)

// APIKeyAuth is a middleware for simple API key authentication
func APIKeyAuth(apiKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		providedKey := c.Get("X-API-Key")
		if providedKey == "" {
			providedKey = c.Query("api_key")
		}

		if providedKey != apiKey {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Unauthorized: Invalid or missing API Key",
			})
		}
		return c.Next()
	}
}
