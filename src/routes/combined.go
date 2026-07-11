package routes

import (
	"context"
	"fmt"
	"os"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

// CombinedHandler godoc
//
//	@Summary		Get combined profile data
//	@Description	Returns the combined SkyCrypt payload for a player's SkyBlock profile, including gear, accessories, pets, skills, dungeons, slayers, collections, minions, bestiary, and other profile sections.
//	@Description	Resource pack preferences supplied by cookie can affect rendered item texture URLs in the response.
//	@ID				getCombinedProfileStats
//	@Tags			Stats
//	@Produce		json
//	@Security		ApiTokenHeader
//	@Param			uuid		path		string	true	"Minecraft UUID or username"	example(4855c53ee4fb4100997600a92fc50984)
//	@Param			profileId	path		string	true	"Hypixel SkyBlock profile UUID or cute name"	example(00912956-3fd6-42ee-a166-3f649ceaf559)
//	@Success		200			{object}	models.CombinedOutput	"Combined profile data returned successfully."
//	@Failure		401			{object}	models.ProcessingError	"X-API-Token is missing or invalid."
//	@Failure		500			{object}	models.ProcessingError	"Player, profile, museum, or combined data could not be loaded."
//	@Router			/api/combined/{uuid}/{profileId} [get]
func CombinedHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Combined")()
	}

	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")
	if len(profileId) > 0 && profileId[0] == '/' {
		profileId = profileId[1:]
	}

	enabledPacks := enabledPacksFromRequest(c)

	reqCtx := c.UserContext()
	cacheKey := responseCacheKey("combined", uuid, profileId, enabledPacksCachePart(enabledPacks))
	if ok, err := sendCachedJSON(c, cacheKey); ok || err != nil {
		return err
	}

	result, err := computeCombinedContext(reqCtx, uuid, profileId, enabledPacks)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	utility.LogVerbose("Returning /api/combined/%s in %s pid=%d", uuid, time.Since(timeNow), os.Getpid())
	return sendAndCacheJSON(c, reqCtx, cacheKey, result, 5*60)
}

func computeCombinedContext(ctx context.Context, uuid string, profileId string, enabledPacks []string) (*models.CombinedOutput, error) {
	var (
		mowojang      *models.MowojangResponse
		profiles      *models.HypixelProfilesResponse
		profile       *skycrypttypes.Profile
		player        *skycrypttypes.Player
		profileMuseum map[string]*skycrypttypes.Museum
		err           error
	)

	isProfileUUID := utility.IsUUID(profileId)
	g, groupCtx := errgroup.WithContext(ctx)
	type museumResult struct {
		profileID string
		museum    map[string]*skycrypttypes.Museum
		err       error
	}
	var selectedMuseumCh chan museumResult

	if isProfileUUID {
		g.Go(func() error {
			var err error
			profileMuseum, err = api.GetMuseumContext(groupCtx, profileId)
			return err
		})
	}

	mowojang, err = api.ResolvePlayerContext(ctx, uuid)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve player: %v", err)
	}

	uuid = mowojang.UUID

	if profileId == "" {
		if cachedProfileID := getCachedSelectedProfileID(ctx, uuid); cachedProfileID != "" {
			selectedMuseumCh = make(chan museumResult, 1)
			g.Go(func() error {
				museum, err := api.GetMuseumContext(groupCtx, cachedProfileID)
				selectedMuseumCh <- museumResult{profileID: cachedProfileID, museum: museum, err: err}
				return nil
			})
		}
	}

	g.Go(func() error {
		var err error
		player, err = api.GetPlayerContext(groupCtx, uuid)
		return err
	})

	g.Go(func() error {
		var err error
		profiles, err = api.GetProfilesContext(groupCtx, uuid)
		if err != nil {
			return err
		}

		profile, err = stats.GetProfile(profiles, profileId)
		if err != nil {
			return err
		}
		if profileId == "" {
			cacheSelectedProfileID(groupCtx, uuid, profile.ProfileID)
		}

		if !isProfileUUID {
			if selectedMuseumCh != nil {
				result := <-selectedMuseumCh
				if result.err == nil && result.profileID == profile.ProfileID {
					profileMuseum = result.museum
					return nil
				}
			}
			profileMuseum, err = api.GetMuseumContext(groupCtx, profile.ProfileID)
			return err
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	members, err := stats.FormatMembersContext(ctx, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to format members: %v", err)
	}

	userProfileValue := profile.Members[uuid]
	museum := profileMuseum[uuid]
	userProfile := &userProfileValue

	return stats.GetCombinedContext(ctx, mowojang, profiles, profile, player, userProfile, museum, members, enabledPacks)
}
