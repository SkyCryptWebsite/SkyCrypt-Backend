package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/stats"
	"time"

	"github.com/gofiber/fiber/v2"
)

// DungeonsHandler godoc
//
//	@Summary		Get dungeons stats of a specified player
//	@Description	Returns dungeons for the given user and profile ID
//	@Tags			dungeons
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.DungeonsOutput
//	@Failure		400			{object}	models.ProcessingError
//	@Router			/api/dungeons/{uuid}/{profileId} [get]
func DungeonsHandler(c *fiber.Ctx) error {
	defer forensics.TrackSpan("handler.Dungeons")()
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

	output := stats.GetDungeons(userProfile)

	fmt.Printf("Returning /api/dungeons/%s in %s\n", profileId, time.Since(timeNow))

	return c.JSON(output)
}
