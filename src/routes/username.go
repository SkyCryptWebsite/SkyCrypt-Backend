package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/utility"
	"time"

	"github.com/gofiber/fiber/v2"
)

// UsernameHandler godoc
// @Summary Get username for a specified UUID
// @Description Returns the username associated with the given UUID
// @Tags username
// @Produce  json
// @Param uuid path string true "UUID"
// @Success 200 {object} models.PlayerResolve
// @Failure 400 {object} models.ProcessingError
// @Router /api/username/{uuid} [get]
func UsernameHandler(c *fiber.Ctx) error {
	timeNow := time.Now()

	uuid := c.Params("uuid")
	if !utility.IsUUID(uuid) {
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}

	username, err := api.GetUsername(uuid)
	if err != nil {
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}

	fmt.Printf("Returning /api/username/%s in %s\n", username, time.Since(timeNow))

	return c.JSON(fiber.Map{
		"displayName": utility.GetDisplayName(username, uuid),
		"username":    username,
		"uuid":        uuid,
	})
}
