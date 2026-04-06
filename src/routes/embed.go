package routes

import (
	"encoding/json"
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/constants"
	redis "skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
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
//	@Param			profileId	query		string	false	"Profile ID (optional, defaults to selected profile)"
//	@Success		200			{object}	models.EmbedData
//	@Failure		400			{object}	models.ProcessingError
//	@Failure		500			{object}	models.ProcessingError
//	@Router			/api/embed/{uuid} [get]
func EmbedHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Embed")()
	}

	timeNow := time.Now()

	uuid := c.Params("uuid")
	isBot := utility.IsKnownBot(c)

	mowojang, err := api.ResolvePlayer(uuid)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to resolve embed player: %v", err),
		})
	}

	profileId := c.Params("profileId")
	if len(profileId) > 0 && profileId[0] == '/' {
		profileId = profileId[1:]
	}

	if utility.IsUUID(profileId) {
		embed, err := redis.Get(fmt.Sprintf("embed:%s:%s", mowojang.UUID, profileId))
		if err == nil && embed != "" {
			var embedData models.EmbedData
			if err := json.Unmarshal([]byte(embed), &embedData); err == nil {
				utility.LogVerbose("Returning /api/embed/%s (cache hit) in %s", profileId, time.Since(timeNow))
				return c.JSON(embedData)
			}
		}

		if isBot {
			utility.LogVerbose("Returning /api/embed/%s (bot, no cache) in %s", profileId, time.Since(timeNow))
			return c.JSON(models.EmbedData{})
		}
	} else if isBot {
		utility.LogVerbose("Returning /api/embed/%s (bot, unresolvable) in %s", profileId, time.Since(timeNow))
		return c.JSON(models.EmbedData{})
	}

	profiles, err := api.GetProfiles(mowojang.UUID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get embed profile: %v", err),
		})
	}

	profile, err := stats.GetProfile(profiles, profileId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get embed profile: %v", err),
		})
	}

	embed, err := redis.Get(fmt.Sprintf("embed:%s:%s", mowojang.UUID, profile.ProfileID))
	if err != nil {
		c.Status(400)
		return c.JSON(constants.InvalidUserError)
	}

	var embedData models.EmbedData
	if err := json.Unmarshal([]byte(embed), &embedData); err != nil {
		return c.JSON(embedData)
	}

	utility.LogVerbose("Returning /api/embed/%s in %s", profileId, time.Since(timeNow))

	return c.JSON(embedData)
}
