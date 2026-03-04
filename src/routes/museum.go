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

func MuseumHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Museum")()
	}

	timeNow := time.Now()

	profileId := c.Params("profileId")
	museum, err := api.GetMuseum(profileId)
	fmt.Print(err)
	if err != nil {
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}

	utility.LogVerbose("Returning /api/museum/%s in %s", profileId, time.Since(timeNow))

	return c.JSON(fiber.Map{
		"museum": museum,
	})
}
