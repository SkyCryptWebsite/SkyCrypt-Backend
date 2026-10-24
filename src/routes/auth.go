package routes

import (
	"crypto/subtle"
	"fmt"
	"os"
	"skycrypt/src/api"
	"skycrypt/src/apikeys"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

// APIKeyMiddleware lets the frontend in with its shared X-API-Token and everyone else with a SkyCrypt API key in
// x-api-key. Keys without the cache-bypass scope only get cached data.
func APIKeyMiddleware(c fiber.Ctx) error {
	if os.Getenv("DISABLE_SERVER_API_AUTH") == "true" {
		c.SetContext(api.WithRequestState(c.Context(), &api.RequestState{}))
		return c.Next()
	}

	if token := strings.TrimSpace(c.Get("X-API-Token")); token != "" {
		expected := strings.TrimSpace(os.Getenv("SERVER_API_TOKEN"))
		if expected == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			return apiKeyError(c, fiber.StatusUnauthorized, "INVALID_API_TOKEN", "Invalid X-API-Token.")
		}
		c.SetContext(api.WithRequestState(c.Context(), &api.RequestState{}))
		return c.Next()
	}

	rawKey := strings.TrimSpace(c.Get("x-api-key"))
	if rawKey == "" {
		return apiKeyError(c, fiber.StatusUnauthorized, "MISSING_API_KEY", "Send your SkyCrypt API key in the x-api-key header.")
	}

	verification, err := apikeys.Verify(c.Context(), rawKey)
	if err != nil {
		fmt.Printf("[APIKEYS] could not verify API key: %v\n", err)
		return apiKeyError(c, fiber.StatusServiceUnavailable, "API_KEY_VERIFICATION_UNAVAILABLE", "API keys can't be verified right now. Try again shortly.")
	}
	if !verification.Valid || verification.Key == nil {
		code, message := "INVALID_API_KEY", "Invalid API key."
		if verification.Error != nil && verification.Error.Code != "" {
			code, message = verification.Error.Code, verification.Error.Message
		}
		return apiKeyError(c, fiber.StatusUnauthorized, code, message)
	}

	usage, err := apikeys.Consume(c.Context(), verification.Key)
	if err != nil {
		fmt.Printf("[APIKEYS] %v\n", err)
		return apiKeyError(c, fiber.StatusServiceUnavailable, "RATE_LIMIT_UNAVAILABLE", "API keys can't be checked right now. Try again shortly.")
	}
	if usage.Limit > 0 {
		c.Set("X-RateLimit-Limit", strconv.FormatInt(usage.Limit, 10))
		c.Set("X-RateLimit-Remaining", strconv.FormatInt(usage.Remaining, 10))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(usage.Reset.Unix(), 10))
	}
	if !usage.Allowed {
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(max(1, int(time.Until(usage.Reset).Seconds()+0.5))))
		return apiKeyError(c, fiber.StatusTooManyRequests, "RATE_LIMITED", "Rate limit exceeded.")
	}

	ctx := api.WithRequestState(c.Context(), &api.RequestState{CacheOnly: !verification.Key.HasScope("cache-bypass")})
	c.SetContext(ctx)
	if err := c.Next(); err != nil {
		return err
	}
	if api.MissedCache(ctx) && c.Response().StatusCode() >= fiber.StatusBadRequest {
		return apiKeyError(c, fiber.StatusNotFound, "NOT_CACHED", "This player isn't cached yet, and your API key can't request fresh data from Hypixel.")
	}
	return nil
}

func apiKeyError(c fiber.Ctx, status int, code string, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"error": message,
		"code":  code,
	})
}
