package routes

import (
	"encoding/json"
	"net/url"
	"os"
	"skycrypt/src/lib"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func enabledPacksFromRequest(c *fiber.Ctx) []string {
	value := ""
	if strings.HasSuffix(c.Path(), "/resolve") {
		value = c.Query("enabledPacks", "")
	}
	if value == "" {
		value = c.Cookies("enabledPacks", "")
	}
	if value == "" && os.Getenv("DEV") == "true" {
		value = c.Query("enabledPacks", "")
	}

	enabledPacks := lib.NormalizeEnabledPacks(parseEnabledPacks(value))
	c.Vary(fiber.HeaderCookie)
	c.Set(fiber.HeaderCacheControl, "private, no-cache")
	return enabledPacks
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
