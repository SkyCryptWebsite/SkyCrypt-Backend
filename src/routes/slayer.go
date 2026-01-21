package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/stats"
	"time"

	"github.com/gofiber/fiber/v2"
)

// SlayersHandler godoc
//
//	@Summary		Get slayer stats of a specified player
//	@Description	Returns slayer statistics for the given user and profile ID
//	@Tags			slayers
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.SlayersOutput
//	@Failure		400			{object}	models.ProcessingError
//	@Router			/api/slayer/{uuid}/{profileId} [get]
func SlayersHandler(c *fiber.Ctx) error {
	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")

	profile, err := api.GetProfile(uuid, profileId)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get profile: %v", err),
		})
	}

	userProfileValue := profile.Members[uuid]
	userProfile := &userProfileValue

	output := stats.GetSlayers(userProfile)

	fmt.Printf("Returning /api/slayer/%s in %s\n", profileId, time.Since(timeNow))

	return c.JSON(output)
}
