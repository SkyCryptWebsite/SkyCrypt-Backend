package routes

import (
	"encoding/json"
	"net/url"
	"os"
	"skycrypt/src/lib"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
)

// PacksInput reads the resource packs a request renders items with, from the enabledPacks cookie the frontend sets.
type PacksInput struct {
	// Documents the cookie only: Resolve reads it through Fiber, which picks the first of duplicate cookies where
	// Huma would pick the last.
	EnabledPacksCookie string `cookie:"enabledPacks" doc:"Resource packs to render items with, as a JSON array or comma-separated list of pack IDs."`
	enabledPacks       []string
}

// Resolve also marks the response as depending on the cookie, errors included, so shared caches don't keep it.
func (p *PacksInput) Resolve(ctx huma.Context) []error {
	c := humafiber.Unwrap(ctx)
	value := c.Cookies("enabledPacks")
	if value == "" && os.Getenv("DEV") == "true" {
		value = ctx.Query("enabledPacks")
	}
	p.enabledPacks = lib.NormalizeEnabledPacks(parseEnabledPacks(value))
	c.Vary(fiber.HeaderCookie)
	ctx.SetHeader(fiber.HeaderCacheControl, "private, no-cache")
	return nil
}

func parseEnabledPacks(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if decoded, err := url.QueryUnescape(value); err == nil {
		value = strings.TrimSpace(decoded)
	}

	var parsed []string
	if json.Unmarshal([]byte(value), &parsed) == nil {
		return parsed
	}

	if strings.HasPrefix(value, "[") || strings.HasSuffix(value, "]") {
		if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
			return nil
		}
		value = strings.TrimSpace(value[1 : len(value)-1])
		if value == "" {
			return nil
		}
	}

	parts := strings.Split(value, ",")
	for index, part := range parts {
		parts[index] = strings.Trim(strings.TrimSpace(part), "'\"")
	}
	return parts
}
