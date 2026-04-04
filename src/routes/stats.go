package routes

import (
	"fmt"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"strings"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/singleflight"
)

var statsGroup singleflight.Group

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
	profileId := normalizeProfileID(c.Params("profileId"))

	// Singleflight: deduplicate concurrent requests for the same player+profile
	sfKey := fmt.Sprintf("stats:%s:%s", uuid, profileId)
	resultIface, err, _ := statsGroup.Do(sfKey, func() (interface{}, error) {
		return computeStats(uuid, profileId)
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	utility.LogVerbose("Returning /api/stats/%s in %s", uuid, time.Since(timeNow))
	return c.JSON(resultIface)
}

func normalizeProfileID(profileId string) string {
	profileId = strings.TrimSpace(profileId)
	profileId = strings.Trim(profileId, "/")
	if profileId == "" {
		return ""
	}

	switch strings.ToLower(profileId) {
	case "undefined", "null":
		return ""
	default:
		return profileId
	}
}

// computeStats fetches all data with maximum pipeline parallelism.
//
// Pipeline strategy:
//   - UUID input:  [ResolvePlayer + GetProfiles + GetPlayer + GetMuseum*] in parallel
//   - Username:    ResolvePlayer first, then [GetProfiles + GetPlayer + GetMuseum*] in parallel
//   - As soon as GetProfiles completes -> start FormatMembers immediately
//     (overlaps with still-running GetPlayer/GetMuseum/ResolvePlayer)
//   - *GetMuseum starts in Phase 1 only when profileId is a UUID (not a cute name)
func computeStats(rawInput string, profileId string) (*models.StatsOutput, error) {
	isUUID := utility.IsUUID(rawInput)
	canFetchMuseumEarly := profileId != "" && utility.IsUUID(profileId)

	var mowojang *models.MowojangReponse

	if !isUUID {
		var err error
		mowojang, err = api.ResolvePlayer(rawInput)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve player: %v", err)
		}
		rawInput = mowojang.UUID
	}
	uuid := rawInput

	type profilesResult struct {
		v *models.HypixelProfilesResponse
		e error
	}
	type playerResult struct {
		v *skycrypttypes.Player
		e error
	}
	type mowojangResult struct {
		v *models.MowojangReponse
		e error
	}
	type museumResult struct {
		v map[string]*skycrypttypes.Museum
		e error
	}
	type membersResult struct {
		v []*models.MemberStats
		e error
	}

	profilesCh := make(chan profilesResult, 1)
	playerCh := make(chan playerResult, 1)

	go func() {
		v, e := api.GetProfiles(uuid)
		profilesCh <- profilesResult{v, e}
	}()
	go func() {
		v, e := api.GetPlayer(uuid)
		playerCh <- playerResult{v, e}
	}()

	// UUID input: resolve mowojang in parallel with everything else
	var mowojangCh chan mowojangResult
	if isUUID {
		mowojangCh = make(chan mowojangResult, 1)
		go func() {
			v, e := api.ResolvePlayer(uuid)
			mowojangCh <- mowojangResult{v, e}
		}()
	}

	// Early museum fetch when profileId is a valid UUID
	var earlyMuseumCh chan museumResult
	if canFetchMuseumEarly {
		earlyMuseumCh = make(chan museumResult, 1)
		go func() {
			v, e := api.GetMuseum(profileId)
			earlyMuseumCh <- museumResult{v, e}
		}()
	}

	// Wait for profiles first
	pr := <-profilesCh
	if pr.e != nil {
		return nil, fmt.Errorf("failed to get profiles: %v", pr.e)
	}
	profiles := pr.v

	profile, err := stats.GetProfile(profiles, profileId)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %v", err)
	}

	// Start FormatMembers immediately (overlaps with remaining in-flight fetches)
	membersCh := make(chan membersResult, 1)
	go func() {
		v, e := stats.FormatMembers(profile)
		membersCh <- membersResult{v, e}
	}()

	// Start museum if not fetched early
	var museumCh chan museumResult
	if canFetchMuseumEarly {
		museumCh = earlyMuseumCh
	} else {
		museumCh = make(chan museumResult, 1)
		go func() {
			v, e := api.GetMuseum(profile.ProfileID)
			museumCh <- museumResult{v, e}
		}()
	}

	// Collect all remaining results
	if isUUID {
		mr := <-mowojangCh
		if mr.e != nil {
			return nil, fmt.Errorf("failed to resolve player: %v", mr.e)
		}
		mowojang = mr.v
	}

	plr := <-playerCh
	if plr.e != nil {
		return nil, fmt.Errorf("failed to get player: %v", plr.e)
	}
	player := plr.v

	musr := <-museumCh
	if musr.e != nil {
		return nil, fmt.Errorf("failed to get museum: %v", musr.e)
	}
	profileMuseum := musr.v

	memr := <-membersCh
	if memr.e != nil {
		return nil, fmt.Errorf("failed to format members: %v", memr.e)
	}
	members := memr.v

	userProfileValue := profile.Members[mowojang.UUID]
	museum := profileMuseum[mowojang.UUID]
	userProfile := &userProfileValue

	return stats.GetStats(mowojang, profiles, profile, player, userProfile, museum, members)
}
