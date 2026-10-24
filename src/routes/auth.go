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

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
)

const apiKeyScheme = "ApiKeyHeader"

// requireAPIKey puts op behind apiKeyMiddleware and documents that in the spec, so the two can't disagree.
func requireAPIKey(humaAPI huma.API, op huma.Operation) huma.Operation {
	op.Security = []map[string][]string{{apiKeyScheme: {}}}
	op.Middlewares = append(op.Middlewares, apiKeyMiddleware)
	documented := errorResponses(humaAPI, map[int]string{
		fiber.StatusUnauthorized:       "The x-api-key header is missing, or the key is invalid, disabled, or expired.",
		fiber.StatusTooManyRequests:    "RATE_LIMITED: the key's rate limit is used up. See the Retry-After header.",
		fiber.StatusServiceUnavailable: "API keys can't be verified right now.",
	})
	if op.Responses == nil {
		op.Responses = map[string]*huma.Response{}
	}
	for status, response := range documented {
		if op.Responses[status] == nil {
			op.Responses[status] = response
		}
	}
	return op
}

// apiKeyMiddleware lets the frontend in with its shared X-API-Token and everyone else with a SkyCrypt API key in
// x-api-key. Keys without the cache-bypass scope only get cached data.
func apiKeyMiddleware(ctx huma.Context, next func(huma.Context)) {
	c := humafiber.Unwrap(ctx)
	if os.Getenv("DISABLE_SERVER_API_AUTH") == "true" {
		next(huma.WithContext(ctx, api.WithRequestState(ctx.Context(), &api.RequestState{})))
		return
	}

	if token := strings.TrimSpace(c.Get("X-API-Token")); token != "" {
		expected := strings.TrimSpace(os.Getenv("SERVER_API_TOKEN"))
		if expected == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			apiKeyError(c, fiber.StatusUnauthorized, "INVALID_API_TOKEN", "Invalid X-API-Token.")
			return
		}
		next(huma.WithContext(ctx, api.WithRequestState(ctx.Context(), &api.RequestState{})))
		return
	}

	rawKey := strings.TrimSpace(c.Get("x-api-key"))
	if rawKey == "" {
		apiKeyError(c, fiber.StatusUnauthorized, "MISSING_API_KEY", "Send your SkyCrypt API key in the x-api-key header.")
		return
	}

	verification, err := apikeys.Verify(ctx.Context(), rawKey)
	if err != nil {
		fmt.Printf("[APIKEYS] could not verify API key: %v\n", err)
		apiKeyError(c, fiber.StatusServiceUnavailable, "API_KEY_VERIFICATION_UNAVAILABLE", "API keys can't be verified right now. Try again shortly.")
		return
	}
	if !verification.Valid || verification.Key == nil {
		code, message := "INVALID_API_KEY", "Invalid API key."
		if verification.Error != nil && verification.Error.Code != "" {
			code, message = verification.Error.Code, verification.Error.Message
		}
		apiKeyError(c, fiber.StatusUnauthorized, code, message)
		return
	}

	usage, err := apikeys.Consume(ctx.Context(), verification.Key)
	if err != nil {
		fmt.Printf("[APIKEYS] %v\n", err)
		apiKeyError(c, fiber.StatusServiceUnavailable, "RATE_LIMIT_UNAVAILABLE", "API keys can't be checked right now. Try again shortly.")
		return
	}
	if usage.Limit > 0 {
		c.Set("X-RateLimit-Limit", strconv.FormatInt(usage.Limit, 10))
		c.Set("X-RateLimit-Remaining", strconv.FormatInt(usage.Remaining, 10))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(usage.Reset.Unix(), 10))
	}
	if !usage.Allowed {
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(max(1, int(time.Until(usage.Reset).Seconds()+0.5))))
		apiKeyError(c, fiber.StatusTooManyRequests, "RATE_LIMITED", "Rate limit exceeded.")
		return
	}

	reqCtx := api.WithRequestState(ctx.Context(), &api.RequestState{CacheOnly: !verification.Key.HasScope("cache-bypass")})
	next(huma.WithContext(ctx, reqCtx))
	// Huma has written the response into Fiber's buffer by now; this replaces it.
	if api.MissedCache(reqCtx) && c.Response().StatusCode() >= fiber.StatusBadRequest {
		apiKeyError(c, fiber.StatusNotFound, "NOT_CACHED", "This player isn't cached yet, and your API key can't request fresh data from Hypixel.")
	}
}

// apiKeyError answers with an APIError. Writing it can only fail if it doesn't encode, which it always does.
func apiKeyError(c fiber.Ctx, status int, code string, message string) {
	_ = c.Status(status).JSON(NewAPIError(status, code, message), "application/problem+json")
}
