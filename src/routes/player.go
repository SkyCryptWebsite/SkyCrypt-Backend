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

func PlayerHandler(c *fiber.Ctx) error {
	defer forensics.TrackSpan("handler.Player")()
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

	player, err := api.GetPlayer(uuid)
	if err != nil {
		c.Status(400)
		return c.JSON(constants.FoundNoPlayerError)
	}

	fmt.Printf("Returning /api/player/%s in %s\n", uuid, time.Since(timeNow))

	return c.JSON(fiber.Map{
		"player": player,
	})
}
