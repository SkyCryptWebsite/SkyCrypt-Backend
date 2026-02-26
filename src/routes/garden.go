package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/constants"
	"skycrypt/src/forensics"
	"skycrypt/src/stats"
	"time"

	"github.com/gofiber/fiber/v2"
)

// GardenHandler godoc
//
//	@Summary		Get garden stats of a specified profile
//	@Description	Returns garden data for the given profile ID
//	@Tags			garden
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//
// @Success		200			{object}	models.Garden
// @Failure		400			{object}	models.ProcessingError
// @Router			/api/garden/{uuid}/{profileId} [get]
func GardenHandler(c *fiber.Ctx) error {
	defer forensics.TrackSpan("handler.Garden")()
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

	garden, err := api.GetGarden(profileId)
	if err != nil {
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}

	output := stats.GetGarden(userProfile, garden)

	fmt.Printf("Returning /api/garden/%s in %s\n", profileId, time.Since(timeNow))

	return c.JSON(output)
}
