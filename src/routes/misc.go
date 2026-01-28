package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/stats"
	"time"

	"github.com/gofiber/fiber/v2"
)

// MiscHandler godoc
//
//	@Summary		Get misc stats of a specified player
//	@Description	Returns misc stats for the given user and profile ID
//	@Tags			misc
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.MiscOutput
//	@Failure		400			{object}	models.ProcessingError
//	@Router			/api/misc/{uuid}/{profileId} [get]
func MiscHandler(c *fiber.Ctx) error {
	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")

	profile, err := api.GetProfile(uuid, profileId)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get profile: %v", err),
		})
	}

	player, err := api.GetPlayer(uuid)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get player: %v", err),
		})
	}

	userProfileValue := profile.Members[uuid]
	userProfile := &userProfileValue

	output := stats.GetMisc(userProfile, profile, player)

	fmt.Printf("Returning /api/misc/%s in %s\n", profileId, time.Since(timeNow))

	return c.JSON(output)
}
