package routes

import (
	"context"
	"skycrypt/src/api"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/stats"
	"skycrypt/src/utility"
	"strings"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

// GardenHandler godoc
//
//	@Summary		Get garden stats
//	@Description	Returns Garden progression, visitors, crop milestones, composter data, plots, upgrades, and related Garden state for a specific SkyBlock profile.
//	@ID				getGardenStats
//	@Tags			Stats
//	@Produce		json
//	@Security		ApiKeyHeader
//	@Param			uuid		path		string	true	"Minecraft UUID of the profile member"	example(4855c53ee4fb4100997600a92fc50984)
//	@Param			profileId	path		string	true	"Hypixel SkyBlock profile UUID or cute name"	example(00912956-3fd6-42ee-a166-3f649ceaf559)
//	@Success		200			{object}	models.Garden			"Garden stats returned successfully."
//	@Failure		401			{object}	models.APIError	"The x-api-key header is missing, or the key is invalid, disabled, or expired."
//	@Failure		404			{object}	models.APIError	"PLAYER_NOT_FOUND or PROFILE_NOT_FOUND: no such player or profile. NOT_CACHED: the player isn't cached, and the key lacks the cache-bypass scope."
//	@Failure		429			{object}	models.APIError	"RATE_LIMITED: the key's rate limit is used up. See the Retry-After header."
//	@Failure		500			{object}	models.APIError	"INTERNAL_ERROR: Unexpected failure."
//	@Failure		502			{object}	models.APIError	"UPSTREAM_ERROR: Hypixel or Mojang failed or sent an unusable response."
//	@Failure		503			{object}	models.APIError	"API keys can't be verified right now."
//	@Failure		504			{object}	models.APIError	"UPSTREAM_TIMEOUT: Hypixel or Mojang didn't answer in time."
//	@Router			/api/garden/{uuid}/{profileId} [get]
func GardenHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Garden")()
	}

	timeNow := time.Now()

	// Members is keyed by the lowercase UUID, and the cache key is lowercased too.
	uuid := strings.ToLower(c.Params("uuid"))
	profileId := c.Params("profileId")
	reqCtx := c.UserContext()
	cacheKey := responseCacheKey("garden", uuid, profileId)
	if ok, err := sendCachedJSON(c, cacheKey); ok || err != nil {
		return err
	}

	body, err := sharedResponse(reqCtx, cacheKey, cacheKey, 60*60, func(ctx context.Context) ([]byte, error) {
		return buildGarden(ctx, uuid, profileId)
	})
	if err != nil {
		return sendBuildError(c, err)
	}

	utility.LogVerbose("Returning /api/garden/%s in %s", profileId, time.Since(timeNow))

	return sendBody(c, reqCtx, cacheKey, body)
}

func buildGarden(ctx context.Context, uuid string, profileId string) ([]byte, error) {
	var (
		profiles *models.HypixelProfilesResponse
		profile  *skycrypttypes.Profile
		garden   *skycrypttypes.Garden
	)

	isProfileUUID := utility.IsUUID(profileId)
	g, groupCtx := errgroup.WithContext(ctx)
	type gardenResult struct {
		profileID string
		garden    *skycrypttypes.Garden
		err       error
	}
	var selectedGardenCh chan gardenResult

	if isProfileUUID {
		g.Go(func() error {
			var err error
			garden, err = api.GetGardenContext(groupCtx, profileId)
			return err
		})
	}

	if profileId == "" {
		if cachedProfileID := getCachedSelectedProfileID(ctx, uuid); cachedProfileID != "" {
			selectedGardenCh = make(chan gardenResult, 1)
			g.Go(func() error {
				fetchedGarden, err := api.GetGardenContext(groupCtx, cachedProfileID)
				selectedGardenCh <- gardenResult{profileID: cachedProfileID, garden: fetchedGarden, err: err}
				return nil
			})
		}
	}

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
			if selectedGardenCh != nil {
				result := <-selectedGardenCh
				if result.err == nil && result.profileID == profile.ProfileID {
					garden = result.garden
					return nil
				}
			}
			garden, err = api.GetGardenContext(groupCtx, profile.ProfileID)
			return err
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, failedTo("Failed to fetch data", err)
	}

	userProfileValue := profile.Members[uuid]
	userProfile := &userProfileValue

	output := stats.GetGarden(userProfile, garden)
	return marshalBody(output)
}
