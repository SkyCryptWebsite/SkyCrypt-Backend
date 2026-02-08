package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/forensics"
	"time"

	"github.com/gofiber/fiber/v2"
)

func MuseumHandler(c *fiber.Ctx) error {
	defer forensics.TrackSpan("handler.Museum")()
	timeNow := time.Now()

	profileId := c.Params("profileId")
	museum, err := api.GetMuseum(profileId)
	fmt.Print(err)
	if err != nil {
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}

	fmt.Printf("Returning /api/museum/%s in %s\n", profileId, time.Since(timeNow))

	return c.JSON(fiber.Map{
		"museum": museum,
	})
}
