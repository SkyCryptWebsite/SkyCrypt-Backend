package routes

import (
	"encoding/json"
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/constants"
	redis "skycrypt/src/db"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"time"

	"github.com/gofiber/fiber/v2"
)

// EmbedHandler godoc
//
//	@Summary		Get embed data for a specified player
//	@Description	Returns embed data for the given user (UUID or username) and optional profile ID
//	@Tags			embed
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID or username"
//	@Param			profileId	path		string	true	"Profile ID (optional, can be empty)"
//	@Success		200			{object}	models.EmbedData
//	@Failure		400			{object}	models.ProcessingError
//	@Failure		500			{object}	models.ProcessingError
//	@Router			/api/embed/{uuid}/{profileId} [get]
func EmbedHandler(c *fiber.Ctx) error {
	timeNow := time.Now()

	uuid := c.Params("uuid")
	mowojang, err := api.ResolvePlayer(uuid)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to resolve player: %v", err),
		})
	}

	profileId := c.Params("profileId")
	if len(profileId) > 0 && profileId[0] == '/' {
		profileId = profileId[1:]
	}

	profiles, err := api.GetProfiles(mowojang.UUID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get profile: %v", err),
		})
	}

	profile, err := stats.GetProfile(profiles, profileId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get profile: %v", err),
		})
	}

	embed, err := redis.Get(fmt.Sprintf("embed:%s:%s", mowojang.UUID, profile.ProfileID))
	if err != nil {
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}

	var embedData models.EmbedData
	if err := json.Unmarshal([]byte(embed), &embedData); err != nil {
		c.Status(500)
		return c.JSON(constants.InternalServerError)
	}

	fmt.Printf("Returning /api/embed/%s in %s\n", profileId, time.Since(timeNow))

	return c.JSON(embedData)
}
