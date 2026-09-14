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

const (
	playerCacheTTL   = 24 * time.Hour
	profilesCacheTTL = 5 * time.Minute
	museumCacheTTL   = 30 * time.Minute
	gardenCacheTTL   = 30 * time.Minute
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
	return player, true
}

func fetchPlayerFresh(ctx context.Context, uuid string) (*skycrypttypes.Player, error) {
	var rawReponse models.HypixelPlayerResponse
	var response skycrypttypes.Player

	body, err := getHypixelBody(ctx, "https://api.hypixel.net/v2/player", url.Values{"uuid": {uuid}})
	if err != nil {
		return &response, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &rawReponse); err != nil {
		return &rawReponse.Player, fmt.Errorf("error parsing JSON: %v", err)
	}

	key := fmt.Sprintf(`player:%s`, uuid)
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

	return &response, true
}

func fetchProfilesFresh(ctx context.Context, uuid string) (*models.HypixelProfilesResponse, error) {
	var response models.HypixelProfilesResponse

	body, err := getHypixelBody(ctx, "https://api.hypixel.net/v2/skyblock/profiles", url.Values{"uuid": {uuid}})
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

	return rawReponse.Members, true
}

func fetchMuseumFresh(ctx context.Context, profileId string) (map[string]*skycrypttypes.Museum, error) {
	var rawReponse models.HypixelMuseumResponse

	body, err := getHypixelBody(ctx, "https://api.hypixel.net/v2/skyblock/museum", url.Values{"profile": {profileId}})
	if err != nil {
		return nil, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &rawReponse); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	key := fmt.Sprintf(`museum:%s`, profileId)
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
	return garden, true
}

func fetchGardenFresh(ctx context.Context, profileId string) (*skycrypttypes.Garden, error) {
	var rawReponse models.HypixelGardenResponse

	body, err := getHypixelBody(ctx, "https://api.hypixel.net/v2/skyblock/garden", url.Values{"profile": {profileId}})
	if err != nil {
		return nil, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(body, &rawReponse); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	key := fmt.Sprintf(`garden:%s`, profileId)
	_ = redis.SetContext(ctx, key, string(body), int(gardenCacheTTL.Seconds()))
	return &rawReponse.Garden, nil
}

func detachedFetchContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), hypixelRequestTimeout)
}

func getHypixelBody(ctx context.Context, endpoint string, query url.Values) ([]byte, error) {
	req, err := buildHypixelRequest(ctx, endpoint, query)
	if err != nil {
		return nil, err
	}

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, security.RedactError(fmt.Errorf("error making request: %w", err))
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
