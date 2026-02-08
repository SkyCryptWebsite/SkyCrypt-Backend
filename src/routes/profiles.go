package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/forensics"
	"skycrypt/src/utility"
	"time"

	"github.com/gofiber/fiber/v2"
)

func ProfilesHandler(c *fiber.Ctx) error {
	defer forensics.TrackSpan("handler.Profiles")()
	timeNow := time.Now()

	uuid := c.Params("uuid")
	if !utility.IsUUID(uuid) {
		tempUUID, err := api.GetUUID(uuid)
		if err != nil {
			c.Status(400)
			return c.JSON(constants.InvalidUserError)
		}
		uuid = tempUUID
	}

	profiles, err := api.GetProfiles(uuid)
	if err != nil {
		c.Status(400)
		return c.JSON(constants.FoundNoProfilesError)
	}

	fmt.Printf("Returning /api/profiles/%s in %s\n", uuid, time.Since(timeNow))

	return c.JSON(fiber.Map{
		"profiles": profiles,
	})
}
