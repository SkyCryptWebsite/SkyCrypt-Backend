package routes

import (
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/utility"
	"time"

	"github.com/gofiber/fiber/v2"
)

// UUIDHandler godoc
//
//	@Summary		Resolve UUID by username
//	@Description	Resolves a Minecraft username to its UUID and returns the normalized player identity payload.
//	@ID				resolveUuidByUsername
//	@Tags			Mojang
//	@Produce		json
//	@Security		ApiTokenHeader
//	@Param			username	path		string	true	"Minecraft username"	minlength(3)	maxlength(16)	example(duckysolucky)
//	@Success		200			{object}	models.PlayerResolve		"UUID resolved successfully."
//	@Failure		400			{object}	models.ProcessingError	"Username is invalid or the player could not be found."
//	@Failure		401			{object}	models.ProcessingError	"X-API-Token is missing or invalid."
//	@Router			/api/uuid/{username} [get]
func UUIDHandler(c *fiber.Ctx) error {
	timeNow := time.Now()
	username := c.Params("username")
	if username == "" || len(username) < 3 || len(username) > 16 {
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}

	uuid, err := api.GetUUID(username)
	if err != nil {
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}

	utility.LogVerbose("Returning /api/username/%s in %s\n", username, time.Since(timeNow))

	return c.JSON(fiber.Map{
		"username": username,
		"uuid":     uuid,
	})
}
