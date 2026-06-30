package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	redis "skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/localcache"
	"skycrypt/src/models"
	"skycrypt/src/utility"
	"strings"
	"time"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

var HYPIXEL_API_KEY = os.Getenv("HYPIXEL_API_KEY")
var hypixelFetchGroup singleflight.Group

const (
	playerCacheTTL       = 24 * time.Hour
	playerCacheRefresh   = 5 * time.Minute
	profilesCacheTTL     = 5 * time.Minute
	profilesCacheRefresh = 1 * time.Minute
	museumCacheTTL       = 30 * time.Minute
	museumCacheRefresh   = 5 * time.Minute
	gardenCacheTTL       = 30 * time.Minute
	gardenCacheRefresh   = 5 * time.Minute
)

func hypixelAPIKey() string {
	if key := strings.TrimSpace(os.Getenv("HYPIXEL_API_KEY")); key != "" {
		return key
	}
	return strings.TrimSpace(HYPIXEL_API_KEY)
}

var (
	playerLocalCache   = localcache.NewLocalCache[*skycrypttypes.Player](128)
	profilesLocalCache = localcache.NewLocalCache[*models.HypixelProfilesResponse](64)
	museumLocalCache   = localcache.NewLocalCache[map[string]*skycrypttypes.Museum](64)
	gardenLocalCache   = localcache.NewLocalCache[*skycrypttypes.Garden](64)
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

	playerKey := fmt.Sprintf(`player:%s`, uuid)
	if player, ok, refresh := playerLocalCache.Get(playerKey); ok {
		if refresh {
			refreshPlayerInBackground(uuid)
		}
		return player, nil
	}

	if player, ok := getPlayerFromCache(ctx, uuid); ok {
		return player, nil
	}

	result, err, _ := hypixelFetchGroup.Do(fmt.Sprintf("player:%s", uuid), func() (interface{}, error) {
		fetchCtx, cancel := detachedFetchContext(ctx)
		defer cancel()

		if player, ok := getPlayerFromCache(fetchCtx, uuid); ok {
			return player, nil
		}

		return fetchPlayerFresh(fetchCtx, uuid)
	})
	if err != nil {
		return &response, err
	}

	return result.(*skycrypttypes.Player), nil
}

func getPlayerFromCache(ctx context.Context, uuid string) (*skycrypttypes.Player, bool) {
	var rawReponse models.HypixelPlayerResponse
	key := fmt.Sprintf(`player:%s`, uuid)
	cache, err := redis.GetContext(ctx, key)
	if err != nil || cache == "" {
		return nil, false
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal([]byte(cache), &rawReponse); err != nil {
		return nil, false
	}

	player := &rawReponse.Player
	playerLocalCache.Set(key, player, playerCacheTTL, playerCacheRefresh)
	return player, true
}

func fetchPlayerFresh(ctx context.Context, uuid string) (*skycrypttypes.Player, error) {
	var rawReponse models.HypixelPlayerResponse
	var response skycrypttypes.Player

	body, err := getHypixelBody(ctx, fmt.Sprintf("https://api.hypixel.net/v2/player?key=%s&uuid=%s", hypixelAPIKey(), uuid))
	if err != nil {
		return &response, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &rawReponse); err != nil {
		return &rawReponse.Player, fmt.Errorf("error parsing JSON: %v", err)
	}

	key := fmt.Sprintf(`player:%s`, uuid)
	playerLocalCache.Set(key, &rawReponse.Player, playerCacheTTL, playerCacheRefresh)
	_ = redis.SetContext(ctx, key, string(body), int(playerCacheTTL.Seconds()))
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

	profilesKey := fmt.Sprintf(`profiles:%s`, uuid)
	if profiles, ok, refresh := profilesLocalCache.Get(profilesKey); ok {
		if refresh {
			refreshProfilesInBackground(uuid)
		}
		return profiles, nil
	}

	if profiles, ok := getProfilesFromCache(ctx, uuid); ok {
		return profiles, nil
	}

	result, err, _ := hypixelFetchGroup.Do(fmt.Sprintf("profiles:%s", uuid), func() (interface{}, error) {
		fetchCtx, cancel := detachedFetchContext(ctx)
		defer cancel()

		if profiles, ok := getProfilesFromCache(fetchCtx, uuid); ok {
			return profiles, nil
		}

		return fetchProfilesFresh(fetchCtx, uuid)
	})
	if err != nil {
		return &response, err
	}

	return result.(*models.HypixelProfilesResponse), nil
}

func getProfilesFromCache(ctx context.Context, uuid string) (*models.HypixelProfilesResponse, bool) {
	var response models.HypixelProfilesResponse
	key := fmt.Sprintf(`profiles:%s`, uuid)
	cache, err := redis.GetContext(ctx, key)
	if err != nil || cache == "" {
		return nil, false
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal([]byte(cache), &response); err != nil {
		return nil, false
	}

	profilesLocalCache.Set(key, &response, profilesCacheTTL, profilesCacheRefresh)
	return &response, true
}

func fetchProfilesFresh(ctx context.Context, uuid string) (*models.HypixelProfilesResponse, error) {
	var response models.HypixelProfilesResponse

	body, err := getHypixelBody(ctx, fmt.Sprintf("https://api.hypixel.net/v2/skyblock/profiles?key=%s&uuid=%s", hypixelAPIKey(), uuid))
	if err != nil {
		return &response, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &response); err != nil {
		return &response, fmt.Errorf("error parsing JSON: %v", err)
	}

	if response.Cause != "" && !response.Success {
		return &response, fmt.Errorf("error fetching profiles: %s", response.Cause)
	}

	key := fmt.Sprintf(`profiles:%s`, uuid)
	profilesLocalCache.Set(key, &response, profilesCacheTTL, profilesCacheRefresh)
	_ = redis.SetContext(ctx, key, string(body), int(profilesCacheTTL.Seconds()))
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
			return &skycrypttypes.Profile{}, fmt.Errorf("no profiles found for UUID %s", uuid)
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
		if profile.ProfileID == targetProfileId || profile.CuteName == targetProfileId {
			return &profile, nil
		}
	}

	return &skycrypttypes.Profile{}, fmt.Errorf("profile with ID %s not found for UUID %s", targetProfileId, uuid)
}

func GetMuseum(profileId string) (map[string]*skycrypttypes.Museum, error) {
	return GetMuseumContext(context.Background(), profileId)
}

func GetMuseumContext(ctx context.Context, profileId string) (map[string]*skycrypttypes.Museum, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.GetMuseum")()
	}

	museumKey := fmt.Sprintf(`museum:%s`, profileId)
	if museum, ok, refresh := museumLocalCache.Get(museumKey); ok {
		if refresh {
			refreshMuseumInBackground(profileId)
		}
		return museum, nil
	}

	if museum, ok := getMuseumFromCache(ctx, profileId); ok {
		return museum, nil
	}

	result, err, _ := hypixelFetchGroup.Do(fmt.Sprintf("museum:%s", profileId), func() (interface{}, error) {
		fetchCtx, cancel := detachedFetchContext(ctx)
		defer cancel()

		if museum, ok := getMuseumFromCache(fetchCtx, profileId); ok {
			return museum, nil
		}

		return fetchMuseumFresh(fetchCtx, profileId)
	})
	if err != nil {
		return nil, err
	}

	return result.(map[string]*skycrypttypes.Museum), nil
}

func getMuseumFromCache(ctx context.Context, profileId string) (map[string]*skycrypttypes.Museum, bool) {
	var rawReponse models.HypixelMuseumResponse
	key := fmt.Sprintf(`museum:%s`, profileId)
	cache, err := redis.GetContext(ctx, key)
	if err != nil || cache == "" {
		return nil, false
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal([]byte(cache), &rawReponse); err != nil {
		return nil, false
	}

	museumLocalCache.Set(key, rawReponse.Members, museumCacheTTL, museumCacheRefresh)
	return rawReponse.Members, true
}

func fetchMuseumFresh(ctx context.Context, profileId string) (map[string]*skycrypttypes.Museum, error) {
	var rawReponse models.HypixelMuseumResponse

	body, err := getHypixelBody(ctx, fmt.Sprintf("https://api.hypixel.net/v2/skyblock/museum?key=%s&profile=%s", hypixelAPIKey(), profileId))
	if err != nil {
		return nil, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &rawReponse); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	key := fmt.Sprintf(`museum:%s`, profileId)
	museumLocalCache.Set(key, rawReponse.Members, museumCacheTTL, museumCacheRefresh)
	_ = redis.SetContext(ctx, key, string(body), int(museumCacheTTL.Seconds()))
	return rawReponse.Members, nil
}

func GetGarden(profileId string) (*skycrypttypes.Garden, error) {
	return GetGardenContext(context.Background(), profileId)
}

func GetGardenContext(ctx context.Context, profileId string) (*skycrypttypes.Garden, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.GetGarden")()
	}

	gardenKey := fmt.Sprintf(`garden:%s`, profileId)
	if garden, ok, refresh := gardenLocalCache.Get(gardenKey); ok {
		if refresh {
			refreshGardenInBackground(profileId)
		}
		return garden, nil
	}

	if garden, ok := getGardenFromCache(ctx, profileId); ok {
		return garden, nil
	}

	result, err, _ := hypixelFetchGroup.Do(fmt.Sprintf("garden:%s", profileId), func() (interface{}, error) {
		fetchCtx, cancel := detachedFetchContext(ctx)
		defer cancel()

		if garden, ok := getGardenFromCache(fetchCtx, profileId); ok {
			return garden, nil
		}

		return fetchGardenFresh(fetchCtx, profileId)
	})
	if err != nil {
		return nil, err
	}

	return result.(*skycrypttypes.Garden), nil
}

func getGardenFromCache(ctx context.Context, profileId string) (*skycrypttypes.Garden, bool) {
	var rawReponse models.HypixelGardenResponse
	key := fmt.Sprintf(`garden:%s`, profileId)
	cache, err := redis.GetContext(ctx, key)
	if err != nil || cache == "" {
		return nil, false
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal([]byte(cache), &rawReponse); err != nil {
		return nil, false
	}

	garden := &rawReponse.Garden
	gardenLocalCache.Set(key, garden, gardenCacheTTL, gardenCacheRefresh)
	return garden, true
}

func fetchGardenFresh(ctx context.Context, profileId string) (*skycrypttypes.Garden, error) {
	var rawReponse models.HypixelGardenResponse

	body, err := getHypixelBody(ctx, fmt.Sprintf("https://api.hypixel.net/v2/skyblock/garden?key=%s&profile=%s", hypixelAPIKey(), profileId))
	if err != nil {
		return nil, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &rawReponse); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	key := fmt.Sprintf(`garden:%s`, profileId)
	gardenLocalCache.Set(key, &rawReponse.Garden, gardenCacheTTL, gardenCacheRefresh)
	_ = redis.SetContext(ctx, key, string(body), int(gardenCacheTTL.Seconds()))
	return &rawReponse.Garden, nil
}

func refreshPlayerInBackground(uuid string) {
	key := fmt.Sprintf(`player:%s`, uuid)
	if !playerLocalCache.StartRefresh(key) {
		return
	}
	go func() {
		defer playerLocalCache.FinishRefresh(key)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = fetchPlayerFresh(ctx, uuid)
	}()
}

func refreshProfilesInBackground(uuid string) {
	key := fmt.Sprintf(`profiles:%s`, uuid)
	if !profilesLocalCache.StartRefresh(key) {
		return
	}
	go func() {
		defer profilesLocalCache.FinishRefresh(key)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = fetchProfilesFresh(ctx, uuid)
	}()
}

func refreshMuseumInBackground(profileId string) {
	key := fmt.Sprintf(`museum:%s`, profileId)
	if !museumLocalCache.StartRefresh(key) {
		return
	}
	go func() {
		defer museumLocalCache.FinishRefresh(key)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = fetchMuseumFresh(ctx, profileId)
	}()
}

func refreshGardenInBackground(profileId string) {
	key := fmt.Sprintf(`garden:%s`, profileId)
	if !gardenLocalCache.StartRefresh(key) {
		return
	}
	go func() {
		defer gardenLocalCache.FinishRefresh(key)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = fetchGardenFresh(ctx, profileId)
	}()
}

func detachedFetchContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
}

func getHypixelBody(ctx context.Context, url string) ([]byte, error) {
	resp, err := getContext(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("received empty response from API")
	}

	return body, nil
}

func getContext(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return HTTPClient.Do(req)
}
