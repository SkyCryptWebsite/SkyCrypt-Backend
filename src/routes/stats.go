package routes

import (
	"context"
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
//	@Summary		Get profile stats
//	@Description	Returns the complete SkyCrypt stats payload for a player on a specific SkyBlock profile.
//	@Description	The player identifier can be a Minecraft UUID or username. The profile identifier can be a Hypixel profile UUID or profile cute name.
//	@ID				getProfileStats
//	@Tags			Stats
//	@Produce		json
//	@Security		ApiTokenHeader
//	@Param			uuid		path		string	true	"Minecraft UUID or username"	example(4855c53ee4fb4100997600a92fc50984)
//	@Param			profileId	path		string	true	"Hypixel SkyBlock profile UUID or cute name"	example(00912956-3fd6-42ee-a166-3f649ceaf559)
//	@Success		200			{object}	models.StatsOutput		"Profile stats returned successfully."
//	@Failure		401			{object}	models.ProcessingError	"X-API-Token is missing or invalid."
//	@Failure		500			{object}	models.ProcessingError	"Player, profile, museum, or stats data could not be loaded."
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
	reqCtx := c.UserContext()
	cacheKey := responseCacheKey("stats", uuid, profileId)
	if ok, err := sendCachedJSON(c, cacheKey); ok || err != nil {
		return err
	}

	output, err := computeStatsContext(reqCtx, uuid, profileId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	utility.LogVerbose("Returning /api/stats/%s in %s", uuid, time.Since(timeNow))
	return sendAndCacheJSON(c, reqCtx, cacheKey, output, 5*60)
}

// SelectedProfileStatsHandler godoc
//
//	@Summary		Get selected profile stats
//	@Description	Returns the complete SkyCrypt stats payload for a player's selected SkyBlock profile.
//	@Description	If Hypixel does not mark a selected profile, the first available profile is used.
//	@ID				getSelectedProfileStats
//	@Tags			Stats
//	@Produce		json
//	@Security		ApiTokenHeader
//	@Param			uuid	path		string	true	"Minecraft UUID or username"	example(4855c53ee4fb4100997600a92fc50984)
//	@Success		200		{object}	models.StatsOutput		"Selected profile stats returned successfully."
//	@Failure		401		{object}	models.ProcessingError	"X-API-Token is missing or invalid."
//	@Failure		500		{object}	models.ProcessingError	"Player, profile, museum, or stats data could not be loaded."
//	@Router			/api/stats/{uuid} [get]
func SelectedProfileStatsHandler(c *fiber.Ctx) error {
	return StatsHandler(c)
}

func computeStatsContext(ctx context.Context, rawInput string, profileId string) (*models.StatsOutput, error) {
	var mowojang *models.MowojangResponse
	var err error
	mowojang, err = api.ResolvePlayerContext(ctx, rawInput)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve player: %v", err)
	}
	uuid := mowojang.UUID
	requestedProfileID := profileId

	type profilesResult struct {
		profiles *models.HypixelProfilesResponse
		err      error
	}
	type playerResult struct {
		player *skycrypttypes.Player
		err    error
	}
	type museumResult struct {
		profileID string
		museum    map[string]*skycrypttypes.Museum
		err       error
	}

	profilesCh := make(chan profilesResult, 1)
	playerCh := make(chan playerResult, 1)
	var selectedMuseumCh chan museumResult

	go func() {
		profiles, fetchErr := api.GetProfilesContext(ctx, uuid)
		profilesCh <- profilesResult{profiles: profiles, err: fetchErr}
	}()

	go func() {
		player, fetchErr := api.GetPlayerContext(ctx, uuid)
		playerCh <- playerResult{player: player, err: fetchErr}
	}()

	if requestedProfileID == "" {
		if cachedProfileID := getCachedSelectedProfileID(ctx, uuid); cachedProfileID != "" {
			selectedMuseumCh = make(chan museumResult, 1)
			go func() {
				museum, fetchErr := api.GetMuseumContext(ctx, cachedProfileID)
				selectedMuseumCh <- museumResult{profileID: cachedProfileID, museum: museum, err: fetchErr}
			}()
		}
	}

	profilesRes := <-profilesCh
	if profilesRes.err != nil {
		return nil, fmt.Errorf("failed to get profiles: %v", profilesRes.err)
	}
	profiles := profilesRes.profiles

	profile, err := stats.GetProfile(profiles, profileId)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %v", err)
	}
	if requestedProfileID == "" {
		cacheSelectedProfileID(ctx, uuid, profile.ProfileID)
	}

	playerRes := <-playerCh
	if playerRes.err != nil {
		return nil, fmt.Errorf("failed to get player: %v", playerRes.err)
	}
	player := playerRes.player

	var profileMuseum map[string]*skycrypttypes.Museum
	if selectedMuseumCh != nil {
		result := <-selectedMuseumCh
		if result.err == nil && result.profileID == profile.ProfileID {
			profileMuseum = result.museum
		}
	}
	if profileMuseum == nil {
		profileMuseum, err = api.GetMuseumContext(ctx, profile.ProfileID)
		if err != nil {
			return nil, fmt.Errorf("failed to get museum: %v", err)
		}
	}

	members, err := stats.FormatMembersContext(ctx, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to format members: %v", err)
	}

	userProfileValue := profile.Members[mowojang.UUID]
	museum := profileMuseum[mowojang.UUID]
	userProfile := &userProfileValue

	return stats.GetStats(mowojang, profiles, profile, player, userProfile, museum, members)
}
