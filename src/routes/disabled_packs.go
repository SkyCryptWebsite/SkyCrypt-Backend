package routes

import (
	"encoding/json"
	"os"
	"skycrypt/src/lib"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func disabledPacksFromRequest(c *fiber.Ctx) []string {
	disabledPacks := []string{}
	disabledPacksCookie := c.Cookies("disabledPacks", "")
	if disabledPacksCookie != "" {
		var parsedPacks []string
		if err := json.Unmarshal([]byte(disabledPacksCookie), &parsedPacks); err == nil {
			disabledPacks = append(disabledPacks, parsedPacks...)
		} else {
			disabledPacks = append(disabledPacks, strings.Split(disabledPacksCookie, ",")...)
		}
	} else if os.Getenv("DEV") == "true" {
		if disabledResourcePacks := c.Query("disabledPacks", ""); disabledResourcePacks != "" {
			disabledPacks = append(disabledPacks, strings.Split(disabledResourcePacks, ",")...)
		}
	}

	return lib.NormalizeDisabledPacks(disabledPacks)
}
