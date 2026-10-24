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
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"golang.org/x/sync/errgroup"
)

func registerGarden(humaAPI huma.API) {
	responses := profileErrors(humaAPI, "Unexpected failure.")
	responses["200"] = jsonResponse[models.Garden](humaAPI, "Garden stats returned successfully.")
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "getGardenStats",
		Method:      fiber.MethodGet,
		Path:        "/api/garden/{uuid}/{profileId}",
		Summary:     "Get garden stats",
		Description: "Returns Garden progression, visitors, crop milestones, composter data, plots, upgrades, and related Garden state for a specific SkyBlock profile.",
		Tags:        []string{"Stats"},
		Responses:   responses,
	}), func(ctx context.Context, in *MemberInput) (*jsonOutput, error) {
		return gardenResponse(ctx, in.UUID, in.ProfileID)
	})
}

func gardenResponse(reqCtx context.Context, uuid string, profileId string) (*jsonOutput, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Garden")()
	}

	timeNow := time.Now()

	// Members is keyed by the lowercase UUID, and the cache key is lowercased too.
	uuid = strings.ToLower(uuid)
	cacheKey := responseCacheKey("garden", uuid, profileId)
	if cached := cachedJSON(reqCtx, cacheKey); cached != nil {
		return cached, nil
	}

	body, err := sharedResponse(reqCtx, cacheKey, cacheKey, 60*60, func(ctx context.Context) ([]byte, error) {
		return buildGarden(ctx, uuid, profileId)
	})
	if err != nil {
		return nil, buildFailure(err)
	}

	utility.LogVerbose("Returning /api/garden/%s in %s", profileId, time.Since(timeNow))

	return jsonBody(reqCtx, cacheKey, body), nil
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
