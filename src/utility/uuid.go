package utility

import (
	"github.com/gofiber/fiber/v2"
)

// IsUUID reports whether uuid is 32 lowercase hex characters (no dashes). It runs several times per request, so it
// checks bytes instead of compiling a regex each call.
func IsUUID(uuid string) bool {
	if len(uuid) != 32 {
		return false
	}
	for i := 0; i < len(uuid); i++ {
		c := uuid[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// IsKnownBot returns true if the request comes from a known bot (e.g. Discordbot, Googlebot).
// The frontend sets the X-Known-Bot header to "true" when it detects the request is from a bot.
func IsKnownBot(c *fiber.Ctx) bool {
	return c.Get("X-Known-Bot") == "true"
}
