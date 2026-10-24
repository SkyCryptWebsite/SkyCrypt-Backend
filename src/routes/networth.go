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
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

func registerNetworth(humaAPI huma.API) {
	responses := profileErrors(humaAPI, "Networth calculation failed.")
	responses["200"] = jsonResponse[models.Networth](humaAPI, "Profile networth returned successfully.")
	responses["503"] = errorResponse(humaAPI, fiber.StatusServiceUnavailable, pricesUnavailableDescription)
	huma.Register(humaAPI, requireAPIKey(humaAPI, huma.Operation{
		OperationID: "getProfileNetworth",
		Method:      fiber.MethodGet,
		Path:        "/api/networth/{uuid}/{profileId}",
		Summary:     "Get profile networth",
		Description: "Calculates normal and non-cosmetic networth for a player's SkyBlock profile.\n" +
			"The player identifier can be a Minecraft UUID or username. The profile identifier can be a Hypixel profile UUID or profile cute name.",
		Tags:      []string{"Stats"},
		Responses: responses,
	}), func(ctx context.Context, in *ProfileInput) (*jsonOutput, error) {
		return networthResponse(ctx, in.UUID, in.ProfileID)
	})
}

func networthResponse(reqCtx context.Context, uuid string, profileId string) (*jsonOutput, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("handler.Networth")()
	}

	timeNow := time.Now()

	// Check the cache under the request as given before resolving the player: a hit then needs no identity lookup,
	// so a cached response still loads when Mojang is down.
	rawCacheKey := responseCacheKey("networth", uuid, profileId)
	if cached := cachedJSON(reqCtx, rawCacheKey); cached != nil {
		return cached, nil
	}

	// Networth only needs the UUID, so skip the Mojang lookup when that is what was given.
	mowojang := &models.MowojangResponse{UUID: uuid}
	if !utility.IsUUID(uuid) {
		resolved, err := api.ResolvePlayerContext(reqCtx, uuid)
		if err != nil {
			return nil, loadError("Failed to resolve player", err)
		}
		mowojang = resolved
	}

	cacheKey := responseCacheKey("networth", mowojang.UUID, profileId)
	if cacheKey.key != rawCacheKey.key {
		if cached := cachedJSON(reqCtx, cacheKey); cached != nil {
			return cached, nil
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
		return nil, buildFailure(err)
	}

	utility.LogVerbose("Returning /api/networth/%s in %s", uuid, time.Since(timeNow))
	return jsonBody(reqCtx, cacheKey, payload), nil
}

// pricesUnavailableDescription documents the 503 of the endpoints that calculate networth.
const pricesUnavailableDescription = "PRICES_UNAVAILABLE or API_KEY_VERIFICATION_UNAVAILABLE: item prices haven't loaded yet, or API keys can't be verified right now."

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
