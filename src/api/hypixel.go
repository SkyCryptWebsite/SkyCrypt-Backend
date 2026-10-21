package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	redis "skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/localcache"
	"skycrypt/src/models"
	"skycrypt/src/security"
	"skycrypt/src/utility"
	"strings"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

var hypixelFetchGroup singleflight.Group

var hypixelRequestTimeout = 30 * time.Second

// Parsed upstream bodies are kept in process memory briefly, so the endpoints one page load calls in parallel (stats,
// networth, combined, inventory, ...) parse a profile once instead of once each. Values are shared between requests
// and must be treated as read-only; singleflight already shares them between concurrent callers.
const parsedCacheTTL = 30 * time.Second

// Parsed profiles can be several MB, and every prefork process has its own caches, so they stay small and expired
// entries are swept rather than held until the key is read again.
var (
	parsedPlayers  = newParsedCache[*skycrypttypes.Player](128)
	parsedProfiles = newParsedCache[*models.HypixelProfilesResponse](16)
	parsedMuseums  = newParsedCache[map[string]*skycrypttypes.Museum](16)
	parsedGardens  = newParsedCache[*skycrypttypes.Garden](16)
)

func newParsedCache[T any](maxEntries int) *localcache.LocalCache[T] {
	cache := localcache.NewLocalCache[T](maxEntries)
	cache.SweepExpiredEvery(parsedCacheTTL)
	return cache
}

// readParsed returns the parsed value for a Redis key, from process memory when it was parsed recently.
func readParsed[T any](ctx context.Context, local *localcache.LocalCache[T], key string, parse func([]byte) (T, error)) (T, bool) {
	if value, ok, _ := local.Get(key); ok {
		return value, true
	}

	var zero T
	body, err := redis.GetBytesContext(ctx, key)
	if err != nil || len(body) == 0 {
		return zero, false
	}
	value, err := parse(body)
	if err != nil {
		return zero, false
	}
	local.Set(key, value, parsedCacheTTL, 0)
	return value, true
}

const (
	playerCacheTTL         = 24 * time.Hour
	profilesCacheTTL       = 5 * time.Minute
	museumCacheTTL         = 30 * time.Minute
	gardenCacheTTL         = 30 * time.Minute
	hypixelRequestAttempts = 2
)

func GetPlayer(uuid string) (*skycrypttypes.Player, error) {
	return GetPlayerContext(context.Background(), uuid)
}

func GetPlayerContext(ctx context.Context, uuid string) (*skycrypttypes.Player, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.GetPlayer")()
	}

	var response skycrypttypes.Player

	if !utility.IsUUID(uuid) {
		respUUID, err := GetUUIDContext(ctx, uuid)
		if err != nil {
			return &response, err
		}

		uuid = respUUID
	}

	if player, ok := getPlayerFromCache(ctx, uuid); ok {
		return player, nil
	}

	result, err := sharedFetch(ctx, fmt.Sprintf("player:%s", uuid), func(fetchCtx context.Context) (*skycrypttypes.Player, error) {
		if player, ok := getPlayerFromCache(fetchCtx, uuid); ok {
			return player, nil
		}

		return fetchPlayerFresh(fetchCtx, uuid)
	})
	if err != nil {
		return &response, err
	}

	return result, nil
}

func getPlayerFromCache(ctx context.Context, uuid string) (*skycrypttypes.Player, bool) {
	return readParsed(ctx, parsedPlayers, fmt.Sprintf(`player:%s`, uuid), parsePlayer)
}

func parsePlayer(body []byte) (*skycrypttypes.Player, error) {
	var rawReponse models.HypixelPlayerResponse
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &rawReponse); err != nil {
		return nil, fmt.Errorf("%w: error parsing Hypixel JSON: %v", ErrUpstream, err)
	}
	return &rawReponse.Player, nil
}

func fetchPlayerFresh(ctx context.Context, uuid string) (*skycrypttypes.Player, error) {
	var rawReponse models.HypixelPlayerResponse
	var response skycrypttypes.Player

	key := fmt.Sprintf(`player:%s`, uuid)
	body, fresh, err := withFallback(ctx, key, func() ([]byte, error) {
		return getHypixelBody(ctx, "https://api.hypixel.net/v2/player", url.Values{"uuid": {uuid}})
	})
	if err != nil {
		return &response, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &rawReponse); err != nil {
		return &rawReponse.Player, fmt.Errorf("%w: error parsing Hypixel JSON: %v", ErrUpstream, err)
	}

	if fresh {
		_ = redis.SetContext(ctx, key, body, int(playerCacheTTL.Seconds()))
		parsedPlayers.Set(key, &rawReponse.Player, parsedCacheTTL, 0)
	}
	if utility.IsForensicsEnabled() {
		forensics.Logger.Info("api_response_parsed",
			zap.String("api", "GetPlayer"),
			zap.String("uuid", uuid),
			zap.Int("response_size_bytes", len(body)),
		)
	}

	return &rawReponse.Player, nil
}

func GetProfiles(uuid string) (*models.HypixelProfilesResponse, error) {
	return GetProfilesContext(context.Background(), uuid)
}

func GetProfilesContext(ctx context.Context, uuid string) (*models.HypixelProfilesResponse, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.GetProfiles")()
	}

	var response models.HypixelProfilesResponse
	if !utility.IsUUID(uuid) {
		respUUID, err := GetUUIDContext(ctx, uuid)
		if err != nil {
			return &response, err
		}

		uuid = respUUID
	}

	if profiles, ok := getProfilesFromCache(ctx, uuid); ok {
		return profiles, nil
	}

	result, err := sharedFetch(ctx, fmt.Sprintf("profiles:%s", uuid), func(fetchCtx context.Context) (*models.HypixelProfilesResponse, error) {
		if profiles, ok := getProfilesFromCache(fetchCtx, uuid); ok {
			return profiles, nil
		}

		return fetchProfilesFresh(fetchCtx, uuid)
	})
	if err != nil {
		return &response, err
	}

	return result, nil
}

func getProfilesFromCache(ctx context.Context, uuid string) (*models.HypixelProfilesResponse, bool) {
	return readParsed(ctx, parsedProfiles, fmt.Sprintf(`profiles:%s`, uuid), parseProfiles)
}

func parseProfiles(body []byte) (*models.HypixelProfilesResponse, error) {
	var response models.HypixelProfilesResponse
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("%w: error parsing Hypixel JSON: %v", ErrUpstream, err)
	}
	return &response, nil
}

func fetchProfilesFresh(ctx context.Context, uuid string) (*models.HypixelProfilesResponse, error) {
	var response models.HypixelProfilesResponse

	key := fmt.Sprintf(`profiles:%s`, uuid)
	body, fresh, err := withFallback(ctx, key, func() ([]byte, error) {
		return getHypixelBody(ctx, "https://api.hypixel.net/v2/skyblock/profiles", url.Values{"uuid": {uuid}})
	})
	if err != nil {
		return &response, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &response); err != nil {
		return &response, fmt.Errorf("%w: error parsing Hypixel JSON: %v", ErrUpstream, err)
	}

	if response.Cause != "" && !response.Success {
		return &response, fmt.Errorf("%w: Hypixel couldn't return profiles: %s", ErrUpstream, response.Cause)
	}

	if fresh {
		_ = redis.SetContext(ctx, key, body, int(profilesCacheTTL.Seconds()))
		parsedProfiles.Set(key, &response, parsedCacheTTL, 0)
	}
	if utility.IsForensicsEnabled() {
		forensics.Logger.Info("api_response_parsed",
			zap.String("api", "GetProfiles"),
			zap.String("uuid", uuid),
			zap.Int("response_size_bytes", len(body)),
		)
	}

	return &response, nil
}

func GetProfile(uuid string, profileId ...string) (*skycrypttypes.Profile, error) {
	return GetProfileContext(context.Background(), uuid, profileId...)
}

func GetProfileContext(ctx context.Context, uuid string, profileId ...string) (*skycrypttypes.Profile, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.GetProfile")()
	}

	profiles, err := GetProfilesContext(ctx, uuid)
	if err != nil {
		return &skycrypttypes.Profile{}, err
	}

	// If no profileId provided, return the first profile or selected profile
	if len(profileId) == 0 || (len(profileId) == 1 && profileId[0] == "") {
		if len(profiles.Profiles) == 0 {
			return &skycrypttypes.Profile{}, fmt.Errorf("%w: no profiles found for UUID %s", ErrProfileNotFound, uuid)
		}

		for _, profile := range profiles.Profiles {
			if profile.Selected {
				return &profile, nil
			}
		}

		return &profiles.Profiles[0], nil
	}

	// If profileId is provided, search for it
	targetProfileId := profileId[0]
	for _, profile := range profiles.Profiles {
		if strings.EqualFold(profile.ProfileID, targetProfileId) || strings.EqualFold(profile.CuteName, targetProfileId) {
			return &profile, nil
		}
	}

	return &skycrypttypes.Profile{}, fmt.Errorf("%w: profile with ID %s not found for UUID %s", ErrProfileNotFound, targetProfileId, uuid)
}

func GetMuseum(profileId string) (map[string]*skycrypttypes.Museum, error) {
	return GetMuseumContext(context.Background(), profileId)
}

func GetMuseumContext(ctx context.Context, profileId string) (map[string]*skycrypttypes.Museum, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.GetMuseum")()
	}

	if museum, ok := getMuseumFromCache(ctx, profileId); ok {
		return museum, nil
	}

	result, err := sharedFetch(ctx, fmt.Sprintf("museum:%s", profileId), func(fetchCtx context.Context) (map[string]*skycrypttypes.Museum, error) {
		if museum, ok := getMuseumFromCache(fetchCtx, profileId); ok {
			return museum, nil
		}

		return fetchMuseumFresh(fetchCtx, profileId)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func getMuseumFromCache(ctx context.Context, profileId string) (map[string]*skycrypttypes.Museum, bool) {
	return readParsed(ctx, parsedMuseums, fmt.Sprintf(`museum:%s`, profileId), parseMuseum)
}

func parseMuseum(body []byte) (map[string]*skycrypttypes.Museum, error) {
	var rawReponse models.HypixelMuseumResponse
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &rawReponse); err != nil {
		return nil, fmt.Errorf("%w: error parsing Hypixel JSON: %v", ErrUpstream, err)
	}
	return rawReponse.Members, nil
}

func fetchMuseumFresh(ctx context.Context, profileId string) (map[string]*skycrypttypes.Museum, error) {
	var rawReponse models.HypixelMuseumResponse

	key := fmt.Sprintf(`museum:%s`, profileId)
	body, fresh, err := withFallback(ctx, key, func() ([]byte, error) {
		return getHypixelBody(ctx, "https://api.hypixel.net/v2/skyblock/museum", url.Values{"profile": {profileId}})
	})
	if err != nil {
		return nil, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &rawReponse); err != nil {
		return nil, fmt.Errorf("%w: error parsing Hypixel JSON: %v", ErrUpstream, err)
	}

	if fresh {
		_ = redis.SetContext(ctx, key, body, int(museumCacheTTL.Seconds()))
		parsedMuseums.Set(key, rawReponse.Members, parsedCacheTTL, 0)
	}
	return rawReponse.Members, nil
}

func GetGarden(profileId string) (*skycrypttypes.Garden, error) {
	return GetGardenContext(context.Background(), profileId)
}

func GetGardenContext(ctx context.Context, profileId string) (*skycrypttypes.Garden, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.GetGarden")()
	}

	if garden, ok := getGardenFromCache(ctx, profileId); ok {
		return garden, nil
	}

	result, err := sharedFetch(ctx, fmt.Sprintf("garden:%s", profileId), func(fetchCtx context.Context) (*skycrypttypes.Garden, error) {
		if garden, ok := getGardenFromCache(fetchCtx, profileId); ok {
			return garden, nil
		}

		return fetchGardenFresh(fetchCtx, profileId)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func getGardenFromCache(ctx context.Context, profileId string) (*skycrypttypes.Garden, bool) {
	return readParsed(ctx, parsedGardens, fmt.Sprintf(`garden:%s`, profileId), parseGarden)
}

func parseGarden(body []byte) (*skycrypttypes.Garden, error) {
	var rawReponse models.HypixelGardenResponse
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &rawReponse); err != nil {
		return nil, fmt.Errorf("%w: error parsing Hypixel JSON: %v", ErrUpstream, err)
	}
	return &rawReponse.Garden, nil
}

func fetchGardenFresh(ctx context.Context, profileId string) (*skycrypttypes.Garden, error) {
	var rawReponse models.HypixelGardenResponse

	key := fmt.Sprintf(`garden:%s`, profileId)
	body, fresh, err := withFallback(ctx, key, func() ([]byte, error) {
		return getHypixelBody(ctx, "https://api.hypixel.net/v2/skyblock/garden", url.Values{"profile": {profileId}})
	})
	if err != nil {
		return nil, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &rawReponse); err != nil {
		return nil, fmt.Errorf("%w: error parsing Hypixel JSON: %v", ErrUpstream, err)
	}

	if fresh {
		_ = redis.SetContext(ctx, key, body, int(gardenCacheTTL.Seconds()))
		parsedGardens.Set(key, &rawReponse.Garden, parsedCacheTTL, 0)
	}
	return &rawReponse.Garden, nil
}

// withFallback stores every good upstream body and serves the last one stored when the upstream fails.
// Cache-only requests never reach the upstream and only get the stored body. fresh is false for stored bodies,
// which callers must not cache as current data.
func withFallback(ctx context.Context, key string, fetch func() ([]byte, error)) (body []byte, fresh bool, err error) {
	state := requestState(ctx)
	if state == nil || !state.CacheOnly {
		body, err = fetch()
		if err == nil {
			redis.SaveFallback(key, body)
			return body, true, nil
		}
	}

	stale, ok := redis.GetFallback(ctx, key)
	if !ok {
		if state != nil && state.CacheOnly {
			state.notCached.Store(true)
			return nil, false, ErrNotCached
		}
		return nil, false, err
	}
	if state != nil {
		state.stale.Store(true)
	}
	if err != nil && utility.IsForensicsEnabled() {
		forensics.Logger.Warn("upstream_fallback_served",
			zap.String("key", key),
			zap.Error(security.RedactError(err)),
		)
	}
	return stale, false, nil
}

// getHypixelBody retries once on a transport error. The transport drops a connection that failed, so the retry
// gets a working one without closing the idle connections every other upstream call shares.
func getHypixelBody(ctx context.Context, endpoint string, query url.Values) ([]byte, error) {
	for attempt := 0; attempt < hypixelRequestAttempts; attempt++ {
		req, err := buildHypixelRequest(ctx, endpoint, query)
		if err != nil {
			return nil, err
		}

		resp, err := HTTPClient.Do(req)
		if err != nil {
			if shouldRetryHypixelRequest(ctx, err, attempt) {
				continue
			}
			return nil, upstreamError("Hypixel", err)
		}

		body, readErr := func() ([]byte, error) {
			defer func() {
				_ = resp.Body.Close()
			}()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("%w: Hypixel returned status code %d", ErrUpstream, resp.StatusCode)
			}

			return io.ReadAll(resp.Body)
		}()
		if readErr != nil {
			if resp.StatusCode == http.StatusOK && shouldRetryHypixelRequest(ctx, readErr, attempt) {
				continue
			}
			if resp.StatusCode == http.StatusOK {
				return nil, upstreamError("Hypixel", readErr)
			}
			return nil, readErr
		}

		if len(body) == 0 {
			return nil, fmt.Errorf("%w: Hypixel sent an empty response", ErrUpstream)
		}

		return body, nil
	}

	return nil, fmt.Errorf("%w: Hypixel failed after %d request attempts", ErrUpstream, hypixelRequestAttempts)
}

func shouldRetryHypixelRequest(ctx context.Context, err error, attempt int) bool {
	if attempt+1 >= hypixelRequestAttempts {
		return false
	}

	return ctx.Err() == nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func buildHypixelRequest(ctx context.Context, endpoint string, query url.Values) (*http.Request, error) {
	key := strings.TrimSpace(os.Getenv("HYPIXEL_API_KEY"))
	if key == "" {
		return nil, errors.New("HYPIXEL_API_KEY is not configured")
	}

	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, security.RedactError(fmt.Errorf("invalid Hypixel endpoint: %w", err))
	}

	encodedQuery := requestURL.Query()
	for name := range encodedQuery {
		if isSensitiveQueryName(name) {
			return nil, errors.New("hypixel request URL contains a sensitive query parameter")
		}
	}
	for name, values := range query {
		if isSensitiveQueryName(name) {
			return nil, errors.New("hypixel request query contains a sensitive parameter")
		}
		for _, value := range values {
			encodedQuery.Add(name, value)
		}
	}
	requestURL.RawQuery = encodedQuery.Encode()
	encodedURL := requestURL.String()
	if strings.Contains(encodedURL, key) {
		return nil, errors.New("hypixel request URL contains the configured API key")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, encodedURL, nil)
	if err != nil {
		return nil, security.RedactError(err)
	}
	req.Header.Set("API-Key", key)
	return req, nil
}

func isSensitiveQueryName(name string) bool {
	switch strings.ToLower(name) {
	case "key", "api_key", "apikey", "token":
		return true
	default:
		return false
	}
}

func getContext(ctx context.Context, requestURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	return HTTPClient.Do(req)
}
