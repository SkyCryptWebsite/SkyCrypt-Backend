package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/stats"
	"time"

	"github.com/gofiber/fiber/v2"
)

// BestiaryHandler godoc
//
//	@Summary		Get bestiary stats of a specified player
//	@Description	Returns bestiary for the given user and profile ID
//	@Tags			bestiary
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.BestiaryOutput
//	@Failure		400			{object}	models.ProcessingError
//	@Router			/api/bestiary/{uuid}/{profileId} [get]
func BestiaryHandler(c *fiber.Ctx) error {
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

	output := stats.GetBestiary(userProfile)

	fmt.Printf("Returning /api/bestiary/%s in %s\n", profileId, time.Since(timeNow))

	return c.JSON(output)
}
