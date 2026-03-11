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

// MinionsHandler godoc
//
//	@Summary		Get minions stats of a specified player
//	@Description	Returns minions for the given user and profile ID
//	@Tags			minions
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.MinionsOutput
//	@Failure		400			{object}	models.ProcessingError
//	@Router			/api/minions/{uuid}/{profileId} [get]
func MinionsHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Minions")()
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

	output := stats.GetMinions(profile)

	utility.LogVerbose("Returning /api/minions/%s in %s", profileId, time.Since(timeNow))

	return c.JSON(output)
}
