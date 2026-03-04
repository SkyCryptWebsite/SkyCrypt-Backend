package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
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
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Slayers")()
	}

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

	utility.LogVerbose("Returning /api/slayer/%s in %s", profileId, time.Since(timeNow))

	return c.JSON(output)
}
