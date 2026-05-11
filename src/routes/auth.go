package routes

import (
	"crypto/subtle"
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func ServerAPITokenMiddleware(c *fiber.Ctx) error {
	if os.Getenv("DISABLE_SERVER_API_AUTH") == "true" {
		return c.Next()
	}

	expectedToken := strings.TrimSpace(os.Getenv("SERVER_API_TOKEN"))
	if expectedToken == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "SERVER_API_TOKEN is not configured",
		})
	}

	providedToken := strings.TrimSpace(c.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(providedToken), "bearer ") {
		providedToken = strings.TrimSpace(providedToken[len("Bearer "):])
	}

	if providedToken == "" {
		providedToken = strings.TrimSpace(c.Get("X-Server-Api-Token"))
	}
	if providedToken == "" {
		providedToken = strings.TrimSpace(c.Get("X-API-Token"))
	}

	fmt.Printf("Received endpoint %s with token: %s\n", c.Path(), providedToken)

	if subtle.ConstantTimeCompare([]byte(providedToken), []byte(expectedToken)) != 1 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized: Invalid API token",
		})
	}

	return c.Next()
}
