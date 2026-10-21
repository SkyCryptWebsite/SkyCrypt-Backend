package routes

import (
	"context"
	"fmt"
	"os"
	"skycrypt/src/api"
	"skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"time"

	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

// NetworthHandler godoc
//
//	@Summary		Get profile networth
//	@Description	Calculates normal and non-cosmetic networth for a player's SkyBlock profile.
//	@Description	The player identifier can be a Minecraft UUID or username. The profile identifier can be a Hypixel profile UUID or profile cute name.
//	@ID				getProfileNetworth
//	@Tags			Stats
//	@Produce		json
//	@Security		ApiKeyHeader
//	@Param			uuid		path		string	true	"Minecraft UUID or username"	example(4855c53ee4fb4100997600a92fc50984)
//	@Param			profileId	path		string	true	"Hypixel SkyBlock profile UUID or cute name"	example(00912956-3fd6-42ee-a166-3f649ceaf559)
//	@Success		200			{object}	models.Networth			"Profile networth returned successfully."
//	@Failure		401			{object}	models.APIError	"The x-api-key header is missing, or the key is invalid, disabled, or expired."
//	@Failure		404			{object}	models.APIError	"PLAYER_NOT_FOUND or PROFILE_NOT_FOUND: no such player or profile. NOT_CACHED: the player isn't cached, and the key lacks the cache-bypass scope."
//	@Failure		429			{object}	models.APIError	"RATE_LIMITED: the key's rate limit is used up. See the Retry-After header."
//	@Failure		500			{object}	models.APIError	"INTERNAL_ERROR: Networth calculation failed."
//	@Failure		502			{object}	models.APIError	"UPSTREAM_ERROR: Hypixel or Mojang failed or sent an unusable response."
//	@Failure		503			{object}	models.APIError	"API keys can't be verified right now, or PRICES_UNAVAILABLE: item prices haven't loaded yet."
//	@Failure		504			{object}	models.APIError	"UPSTREAM_TIMEOUT: Hypixel or Mojang didn't answer in time."
//	@Router			/api/networth/{uuid}/{profileId} [get]
func NetworthHandler(c *fiber.Ctx) error {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Networth")()
	}

	timeNow := time.Now()

	uuid := c.Params("uuid")
	profileId := c.Params("profileId")
	if len(profileId) > 0 && profileId[0] == '/' {
		profileId = profileId[1:]
	}
	reqCtx := c.UserContext()

	// Check the cache under the request as given before resolving the player: a hit then needs no identity lookup,
	// so a cached response still loads when Mojang is down.
	rawCacheKey := responseCacheKey("networth", uuid, profileId)
	if ok, err := sendCachedJSON(c, rawCacheKey); ok || err != nil {
		return err
	}

	// Networth only needs the UUID, so skip the Mojang lookup when that is what was given.
	mowojang := &models.MowojangResponse{UUID: uuid}
	if !utility.IsUUID(uuid) {
		resolved, err := api.ResolvePlayerContext(reqCtx, uuid)
		if err != nil {
			return sendLoadError(c, "Failed to resolve player", err)
		}
		mowojang = resolved
	}

	cacheKey := responseCacheKey("networth", mowojang.UUID, profileId)
	if cacheKey.key != rawCacheKey.key {
		if ok, err := sendCachedJSON(c, cacheKey); ok || err != nil {
			return err
		}
	}

	payload, err := sharedResponse(reqCtx, cacheKey, rawCacheKey, 5*60, func(ctx context.Context) ([]byte, error) {
		data, err := loadProfileData(ctx, mowojang, profileId, profileParts{museum: true})
		if err != nil {
			return nil, failedTo("Failed to fetch data", err)
		}
		payload, _, err := profileNetworth(ctx, data)
		if err != nil {
			return nil, failedTo("Failed to create networth calculator", err)
		}
		return payload, nil
	})
	if err != nil {
		return sendBuildError(c, err)
	}

	utility.LogVerbose("Returning /api/networth/%s in %s", uuid, time.Since(timeNow))
	return sendBody(c, reqCtx, cacheKey, payload)
}

const networthCacheTTLSeconds = 5 * 60

// networthTotals are the two totals /embed shows.
type networthTotals struct {
	Normal      float64
	NonCosmetic float64
}

// profileNetworth calculates normal and non-cosmetic networth for a profile member, cached by resolved IDs so
// /networth (under any spelling of the player or profile) and /embed share one calculation and show the same
// number. It returns the /networth response body and its totals.
func profileNetworth(ctx context.Context, data *profileData) ([]byte, networthTotals, error) {
	cacheKey := fmt.Sprintf("networth:%s:%s", data.Profile.ProfileID, data.Mowojang.UUID)
	if processedResponseCacheEnabled() {
		if cached, err := db.GetBytesContext(ctx, cacheKey); err == nil && len(cached) > 0 {
			if totals, err := parseNetworthTotals(cached); err == nil {
				return cached, totals, nil
			}
		}
	}

	start := time.Now()
	var bankBalance float64
	if data.Profile.Banking != nil && data.Profile.Banking.Balance != nil {
		bankBalance = *data.Profile.Banking.Balance
	}

	calculator, err := skyhelpernetworthgo.NewProfileNetworthCalculator(data.Member(), data.Museum(), bankBalance)
	if err != nil {
		return nil, networthTotals{}, err
	}

	networth, nonCosmeticNetworth := calculator.GetNetworthAndNonCosmetic(skyhelpernetworthgo.NetworthOptions{OnlyNetworth: true})

	if utility.IsForensicsEnabled() {
		// Enough to explain a wrong number later: which prices, which data, and where the total came from.
		categoryTotals := make(map[string]float64, len(networth.Types))
		for categoryID, category := range networth.Types {
			categoryTotals[categoryID] = category.Total
		}
		forensics.Logger.Info("networth_calculated",
			zap.String("uuid", data.Mowojang.UUID),
			zap.String("profile_id", data.Profile.ProfileID),
			zap.Float64("normal", networth.Networth),
			zap.Float64("non_cosmetic", nonCosmeticNetworth.Networth),
			zap.Int64("prices_updated_unix", int64(calculator.Prices["lastUpdated"])),
			zap.Bool("stale_profile_data", api.ServedStale(ctx)),
			zap.Any("category_totals", categoryTotals),
			zap.Duration("duration", time.Since(start)),
			zap.Int("pid", os.Getpid()),
		)
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	payload, err := json.Marshal(fiber.Map{
		"normal":      networth,
		"nonCosmetic": nonCosmeticNetworth,
	})
	if err != nil {
		return nil, networthTotals{}, fmt.Errorf("failed to marshal networth: %v", err)
	}

	if processedResponseCacheEnabled() && !api.ServedStale(ctx) {
		cacheCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		_ = db.SetContext(cacheCtx, cacheKey, payload, networthCacheTTLSeconds)
	}

	return payload, networthTotals{Normal: networth.Networth, NonCosmetic: nonCosmeticNetworth.Networth}, nil
}

func parseNetworthTotals(payload []byte) (networthTotals, error) {
	var parsed struct {
		Normal struct {
			Networth float64 `json:"networth"`
		} `json:"normal"`
		NonCosmetic struct {
			Networth float64 `json:"networth"`
		} `json:"nonCosmetic"`
	}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return networthTotals{}, err
	}
	return networthTotals{Normal: parsed.Normal.Networth, NonCosmetic: parsed.NonCosmetic.Networth}, nil
}
