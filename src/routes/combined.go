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

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
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

	disabledPacks := []string{}
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

func computeCombined(uuid string, profileId string, disabledPacks []string) (*models.CombinedOutput, error) {
	type profilesResult struct {
		profiles *models.HypixelProfilesResponse
		err      error
	}
	type playerResult struct {
		player *skycrypttypes.Player
		err    error
	}
	type museumResult struct {
		museum map[string]*skycrypttypes.Museum
		err    error
	}

	profilesCh := make(chan profilesResult, 1)
	playerCh := make(chan playerResult, 1)
	museumCh := make(chan museumResult, 1)

	go func() {
		profiles, fetchErr := api.GetProfiles(uuid)
		profilesCh <- profilesResult{profiles: profiles, err: fetchErr}
	}()

	go func() {
		player, fetchErr := api.GetPlayer(uuid)
		playerCh <- playerResult{player: player, err: fetchErr}
	}()

	go func() {
		museum, fetchErr := api.GetMuseum(profileId)
		museumCh <- museumResult{museum: museum, err: fetchErr}
	}()

	profilesRes := <-profilesCh
	if profilesRes.err != nil {
		return nil, fmt.Errorf("failed to get profiles: %v", profilesRes.err)
	}
	profiles := profilesRes.profiles

	profile, err := stats.GetProfile(profiles, profileId)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %v", err)
	}

	playerRes := <-playerCh
	if playerRes.err != nil {
		return nil, fmt.Errorf("failed to get player: %v", playerRes.err)
	}
	player := playerRes.player

	museumRes := <-museumCh
	if museumRes.err != nil {
		return nil, fmt.Errorf("failed to get museum: %v", museumRes.err)
	}
	profileMuseum := museumRes.museum

	members, err := stats.FormatMembers(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to format members: %v", err)
	}

	userProfileValue := profile.Members[uuid]
	museum := profileMuseum[uuid]
	userProfile := &userProfileValue
	mowojang := &models.MowojangResponse{UUID: uuid}

	return stats.GetCombined(mowojang, profiles, profile, player, userProfile, museum, members, disabledPacks)
}
