package routes

import (
	"encoding/json"
	"fmt"
	"os"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// CombinedHandler godoc
//
//	@Summary		Get combined stats of a specified player
//	@Description	Returns combined  stats for the given user and profile ID
//	@Tags			combinedStats
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.CombinedOutput
//	@Failure		400			{object}	models.ProcessingError
//	@Failure		500			{object}	models.ProcessingError
//	@Router			/api/combined/{uuid}/{profileId} [get]
func CombinedHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Stats")()
	}

	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")
	if len(profileId) > 0 && profileId[0] == '/' {
		profileId = profileId[1:]
	}

	disabledPacks := []string{""}
	disabledPacksCookies := c.Cookies("disabledPacks", "FAILED")
	if disabledPacksCookies != "FAILED" {
		var parsedPacks []string
		err := json.Unmarshal([]byte(disabledPacksCookies), &parsedPacks)
		if err == nil {
			disabledPacks = append(disabledPacks, parsedPacks...)
		}
	} else if os.Getenv("DEV") == "true" {
		disabledResourcePacks := c.Query("disabledPacks", "")
		if disabledResourcePacks != "" {
			disabledPacks = strings.Split(disabledResourcePacks, ",")
		}
	}

	result, err := computeCombined(uuid, profileId, disabledPacks)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	utility.LogVerbose("Returning /api/combined/%s in %s", uuid, time.Since(timeNow))
	return c.JSON(result)
}

func computeCombined(rawInput string, profileId string, disabledPacks []string) (*models.CombinedOutput, error) {
	var err error
	var mowojang *models.MowojangReponse
	mowojang, err = api.ResolvePlayer(rawInput)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve player: %v", err)
	}
	uuid := mowojang.UUID

	profiles, err := api.GetProfiles(uuid)
	if err != nil {
		return nil, fmt.Errorf("failed to get profiles: %v", err)
	}

	profile, err := stats.GetProfile(profiles, profileId)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %v", err)
	}

	player, err := api.GetPlayer(uuid)
	if err != nil {
		return nil, fmt.Errorf("failed to get player: %v", err)
	}

	profileMuseum, err := api.GetMuseum(profile.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get museum: %v", err)
	}

	members, err := stats.FormatMembers(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to format members: %v", err)
	}

	userProfileValue := profile.Members[mowojang.UUID]
	museum := profileMuseum[mowojang.UUID]
	userProfile := &userProfileValue

	return stats.GetCombined(mowojang, profiles, profile, player, userProfile, museum, members, disabledPacks)
}
