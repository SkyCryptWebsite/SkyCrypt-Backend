package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/stats"
	"time"

	"github.com/gofiber/fiber/v2"
)

// CollectionsHandler godoc
//
//	@Summary		Get collections stats of a specified player
//	@Description	Returns collections for the given user and profile ID
//	@Tags			collections
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.CollectionsOutput
//	@Failure		500			{object}	models.ProcessingError
//	@Router			/api/collections/{uuid}/{profileId} [get]
func CollectionsHandler(c *fiber.Ctx) error {
	defer forensics.TrackSpan("handler.Collections")()
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

	output := stats.GetCollections(userProfile, profile)

	fmt.Printf("Returning /api/collections/%s in %s\n", profileId, time.Since(timeNow))

	return c.JSON(output)
}
