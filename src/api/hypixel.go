package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	redis "skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/utility"

	skycrypttypes "github.com/DuckySoLucky/SkyCrypt-Types"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

var HYPIXEL_API_KEY = os.Getenv("HYPIXEL_API_KEY")

func GetPlayer(uuid string) (*skycrypttypes.Player, error) {
	return GetPlayerContext(context.Background(), uuid)
}

func GetPlayerContext(ctx context.Context, uuid string) (*skycrypttypes.Player, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.GetPlayer")()
	}

	var rawReponse models.HypixelPlayerResponse
	var response skycrypttypes.Player

	if !utility.IsUUID(uuid) {
		respUUID, err := GetUUIDContext(ctx, uuid)
		if err != nil {
			return &response, err
		}

		uuid = respUUID
	}

	cache, err := redis.GetContext(ctx, fmt.Sprintf(`player:%s`, uuid))
	if err == nil && cache != "" {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal([]byte(cache), &rawReponse)
		if err == nil {
			return &rawReponse.Player, nil
		}
	}

	resp, err := getContext(ctx, fmt.Sprintf("https://api.hypixel.net/v2/player?key=%s&uuid=%s", HYPIXEL_API_KEY, uuid))

	if err != nil {
		return &response, fmt.Errorf("error making request: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return &response, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &response, fmt.Errorf("error reading response: %v", err)
	}

	if len(body) == 0 {
		return &response, fmt.Errorf("received empty response from API")
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(body, &rawReponse)
	if err != nil {
		return &rawReponse.Player, fmt.Errorf("error parsing JSON: %v", err)
	}

	_ = redis.SetContext(ctx, fmt.Sprintf(`player:%s`, uuid), string(body), 24*60*60)
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

	cache, err := redis.GetContext(ctx, fmt.Sprintf(`profiles:%s`, uuid))
	if err == nil && cache != "" {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal([]byte(cache), &response)
		if err == nil {
			return &response, nil
		}
	}

	resp, err := getContext(ctx, fmt.Sprintf("https://api.hypixel.net/v2/skyblock/profiles?key=%s&uuid=%s", HYPIXEL_API_KEY, uuid))
	if err != nil {
		return &response, fmt.Errorf("error making request: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return &response, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &response, fmt.Errorf("error reading response: %v", err)
	}

	if len(body) == 0 {
		return &response, fmt.Errorf("received empty response from API")
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(body, &response)
	if err != nil {
		return &response, fmt.Errorf("error parsing JSON: %v", err)
	}

	if response.Cause != "" && !response.Success {
		return &response, fmt.Errorf("error fetching profiles: %s", response.Cause)
	}

	_ = redis.SetContext(ctx, fmt.Sprintf(`profiles:%s`, uuid), string(body), 5*60) // Cache for 5 minutes
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

	var rawReponse models.HypixelMuseumResponse

	cache, err := redis.GetContext(ctx, fmt.Sprintf(`museum:%s`, profileId))
	if err == nil && cache != "" {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal([]byte(cache), &rawReponse)
		if err == nil {
			return rawReponse.Members, nil
		}
	}

	resp, err := getContext(ctx, fmt.Sprintf("https://api.hypixel.net/v2/skyblock/museum?key=%s&profile=%s", HYPIXEL_API_KEY, profileId))
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

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(body, &rawReponse)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	_ = redis.SetContext(ctx, fmt.Sprintf(`museum:%s`, profileId), string(body), 60*30) // Cache for 30 minutes
	return rawReponse.Members, nil
}

func GetGarden(profileId string) (*skycrypttypes.Garden, error) {
	return GetGardenContext(context.Background(), profileId)
}

func GetGardenContext(ctx context.Context, profileId string) (*skycrypttypes.Garden, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.GetGarden")()
	}

	var rawReponse models.HypixelGardenResponse

	cache, err := redis.GetContext(ctx, fmt.Sprintf(`garden:%s`, profileId))
	if err == nil && cache != "" {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal([]byte(cache), &rawReponse)
		if err == nil {
			return &rawReponse.Garden, nil
		}
	}

	resp, err := getContext(ctx, fmt.Sprintf("https://api.hypixel.net/v2/skyblock/garden?key=%s&profile=%s", HYPIXEL_API_KEY, profileId))
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

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(body, &rawReponse)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	_ = redis.SetContext(ctx, fmt.Sprintf(`garden:%s`, profileId), string(body), 60*30) // Cache for 30 minutes
	return &rawReponse.Garden, nil
}

func getContext(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return HTTPClient.Do(req)
}
