package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	"github.com/gofiber/fiber/v2"
)

// StatsHandler godoc
//
//	@Summary		Get stats of a specified player
//	@Description	Returns stats for the given user and profile ID
//	@Tags			stats
//	@Produce		json
//	@Param			uuid		path		string	true	"User UUID"
//	@Param			profileId	path		string	true	"Profile ID"
//	@Success		200			{object}	models.StatsOutput
//	@Failure		400			{object}	models.ProcessingError
//	@Failure		500			{object}	models.ProcessingError
//	@Router			/api/stats/{uuid}/{profileId} [get]
func StatsHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Stats")()
	}

	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")
	if len(profileId) > 0 && profileId[0] == '/' {
		profileId = profileId[1:]
	}

	output, err := computeStats(uuid, profileId)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	utility.LogVerbose("Returning /api/stats/%s in %s", uuid, time.Since(timeNow))
	return c.JSON(output)
}

func computeStats(rawInput string, profileId string) (*models.StatsOutput, error) {
	var mowojang *models.MowojangReponse
	var err error
	mowojang, err = api.ResolvePlayer(rawInput)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve player: %v", err)
	}
	uuid := mowojang.UUID

	type profilesResult struct {
		profiles *models.HypixelProfilesResponse
		err      error
	}
	type playerResult struct {
		player *skycrypttypes.Player
		err    error
	}

	profilesCh := make(chan profilesResult, 1)
	playerCh := make(chan playerResult, 1)

	go func() {
		profiles, fetchErr := api.GetProfiles(uuid)
		profilesCh <- profilesResult{profiles: profiles, err: fetchErr}
	}()

	go func() {
		player, fetchErr := api.GetPlayer(uuid)
		playerCh <- playerResult{player: player, err: fetchErr}
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

	return stats.GetStats(mowojang, profiles, profile, player, userProfile, museum, members)
}
