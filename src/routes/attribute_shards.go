package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/stats"
	"time"

	"github.com/gofiber/fiber/v2"
)

// AttributeShardsHandler godoc
//
//	@Summary		Get attribute shards stats of a specified player
//	@Description	Returns attribute shards stats for the given user and profile ID
//	@Tags			attribute_shards
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.AttributeShardsOutput
//	@Failure		400			{object}	models.ProcessingError
//	@Router			/api/attribute_shards/{uuid}/{profileId} [get]
func AttributeShardsHandler(c *fiber.Ctx) error {
	defer forensics.TrackSpan("handler.AttributeShards")()
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

	output := stats.GetAttributeShards(userProfile)

	fmt.Printf("Returning /api/attribute_shards/%s in %s\n", profileId, time.Since(timeNow))

	return c.JSON(output)
}
