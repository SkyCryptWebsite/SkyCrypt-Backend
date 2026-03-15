package utility

import (
	"regexp"

	"github.com/gofiber/fiber/v2"
)

func IsUUID(uuid string) bool {
	noDashMatch, _ := regexp.MatchString(`^[0-9a-f]{32}$`, uuid)
	return noDashMatch
}

// IsKnownBot returns true if the request comes from a known bot (e.g. Discordbot, Googlebot).
// The frontend sets the X-Known-Bot header to "true" when it detects the request is from a bot.
func IsKnownBot(c *fiber.Ctx) bool {
	return false
	// return c.Get("X-Known-Bot") == "true"
}
