package api

import (
	"fmt"
	"io"
	redis "skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/models"
	"skycrypt/src/utility"

	"net/http"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"golang.org/x/sync/singleflight"
)

var resolvePlayerGroup singleflight.Group

func GetUUID(username string, throwAnError ...bool) (string, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.GetUUID")()
	}

	shouldThrowError := true
	if len(throwAnError) > 0 {
		shouldThrowError = throwAnError[0]
	}

	var post models.MowojangResponse

	cache, err := redis.Get(fmt.Sprintf("uuid:%s", strings.ToLower(username)))
	if err == nil && cache != "" {
		return cache, nil
	}

	resp, err := HTTPClient.Get(fmt.Sprintf("https://mowojang.matdoes.dev/%s", username))
	if err != nil || resp.StatusCode != http.StatusOK {
		// Fallback to official Mojang API
		if resp != nil {
			_ = resp.Body.Close()
		}
		return getUUIDFromMojang(username, shouldThrowError)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if shouldThrowError {
			return post.UUID, fmt.Errorf("error reading response: %v", err)
		}
		return "", nil
	}

	if string(body) == "player not found" {
		// Fallback to official Mojang API
		return getUUIDFromMojang(username, shouldThrowError)
	}

	if len(body) == 0 {
		if shouldThrowError {
			return post.UUID, fmt.Errorf("received empty response from API")
		}
		return "", nil
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(body, &post)
	if err != nil {
		if shouldThrowError {
			return post.UUID, fmt.Errorf("error parsing JSON: %v", err)
		}
		return "", nil
	}

	_ = redis.Set(fmt.Sprintf("uuid:%s", strings.ToLower(post.Name)), post.UUID, 24*60*60) // Cache for 24 hours
	_ = redis.Set(fmt.Sprintf("username:%s", post.UUID), post.Name, 24*60*60)              // Cache for 24 hours
	_ = redis.Set(fmt.Sprintf("mowojang:%s", post.UUID), string(body), 24*60*60)           // Cross-cache for ResolvePlayer

	return post.UUID, nil
}

func GetUsername(uuid string, throwAnError ...bool) (string, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.GetUsername")()
	}

	shouldThrowError := true
	if len(throwAnError) > 0 {
		shouldThrowError = throwAnError[0]
	}

	var post models.MowojangResponse

	cache, err := redis.Get(fmt.Sprintf("username:%s", uuid))
	if err == nil && cache != "" {
		return cache, nil
	}

	resp, err := HTTPClient.Get(fmt.Sprintf("https://mowojang.matdoes.dev/%s", uuid))
	if err != nil || resp.StatusCode != http.StatusOK {
		// Fallback to official Mojang API
		if resp != nil {
			_ = resp.Body.Close()
		}
		return getUsernameFromMojang(uuid, shouldThrowError)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if shouldThrowError {
			return post.Name, fmt.Errorf("error reading response: %v", err)
		}
		return "", nil
	}

	if string(body) == "player not found" {
		// Fallback to official Mojang API
		return getUsernameFromMojang(uuid, shouldThrowError)
	}

	if len(body) == 0 {
		if shouldThrowError {
			return post.Name, fmt.Errorf("received empty response from API")
		}
		return "", nil
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(body, &post)
	if err != nil {
		if shouldThrowError {
			return post.Name, fmt.Errorf("error parsing JSON: %v", err)
		}
		return "", nil
	}

	_ = redis.Set(fmt.Sprintf("uuid:%s", strings.ToLower(post.Name)), uuid, 24*60*60) // Cache for 24 hours
	_ = redis.Set(fmt.Sprintf("username:%s", uuid), post.Name, 24*60*60)              // Cache for 24 hours
	_ = redis.Set(fmt.Sprintf("mowojang:%s", uuid), string(body), 24*60*60)           // Cross-cache for ResolvePlayer

	return post.Name, nil
}

func ResolvePlayer(input string, throwAnError ...bool) (*models.MowojangResponse, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.ResolvePlayer")()
	}

	shouldThrowError := true
	if len(throwAnError) > 0 {
		shouldThrowError = throwAnError[0]
	}

	var post models.MowojangResponse
	uuid := input
	if !utility.IsUUID(uuid) {
		tempUUID, err := GetUUID(uuid, shouldThrowError)
		if err != nil {
			if shouldThrowError {
				return &post, fmt.Errorf("error resolving UUID for username '%s': %v", uuid, err)
			}
			return &post, nil
		}
		uuid = tempUUID
	}

	// Singleflight: deduplicate concurrent resolutions for the same UUID
	// (e.g. handler + FormatMembers resolving the same player simultaneously)
	resultIface, err, _ := resolvePlayerGroup.Do(uuid, func() (interface{}, error) {
		return resolvePlayerByUUID(uuid)
	})
	if err != nil {
		if shouldThrowError {
			return &post, err
		}
		return &post, nil
	}

	return resultIface.(*models.MowojangResponse), nil
}

func resolvePlayerByUUID(uuid string) (*models.MowojangResponse, error) {
	var post models.MowojangResponse

	cache, err := redis.Get(fmt.Sprintf("mowojangUUID:%s", uuid))
	if err == nil && cache != "" {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal([]byte(cache), &post)
		if err == nil {
			return &post, nil
		}
	}

	cache, err = redis.Get(fmt.Sprintf("mowojangUsername:%s", uuid))
	if err == nil && cache != "" {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal([]byte(cache), &post)
		if err == nil {
			return &post, nil
		}
	}

	resp, err := HTTPClient.Get(fmt.Sprintf("https://mowojang.matdoes.dev/%s", uuid))
	if err != nil || resp.StatusCode != http.StatusOK {
		// Fallback to official Mojang API
		if resp != nil {
			_ = resp.Body.Close()
		}
		return resolvePlayerFromMojang(uuid)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &post, fmt.Errorf("error reading response: %v", err)
	}

	if string(body) == "player not found" {
		// Fallback to official Mojang API
		return resolvePlayerFromMojang(uuid)
	}

	if len(body) == 0 {
		return &post, fmt.Errorf("received empty response from API")
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(body, &post)
	if err != nil {
		return &post, fmt.Errorf("error parsing JSON: %v", err)
	}

	_ = redis.Set(fmt.Sprintf("mowojangUUID:%s", uuid), string(body), 24*60*60)                           // Cache for 24 hours
	_ = redis.Set(fmt.Sprintf("mowojangUsername:%s", strings.ToLower(post.Name)), string(body), 24*60*60) // Cache for 24 hours
	_ = redis.Set(fmt.Sprintf("uuid:%s", strings.ToLower(post.Name)), post.UUID, 24*60*60)                // Cross-cache for GetUUID
	_ = redis.Set(fmt.Sprintf("username:%s", post.UUID), post.Name, 24*60*60)                             // Cross-cache for GetUsername

	return &post, nil
}

func getUUIDFromMojang(username string, shouldThrowError bool) (string, error) {
	resp, err := HTTPClient.Get(fmt.Sprintf("https://api.mojang.com/users/profiles/minecraft/%s", username))
	if err != nil {
		if shouldThrowError {
			return "", fmt.Errorf("error making request to Mojang API: %v", err)
		}
		return "", nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if shouldThrowError {
			return "", fmt.Errorf("error reading Mojang API response: %v", err)
		}
		return "", nil
	}

	if resp.StatusCode == http.StatusNotFound {
		if shouldThrowError {
			return "", fmt.Errorf("invalid username or UUID provided")
		}
		return "Player not Found", nil
	}

	if len(body) == 0 {
		if shouldThrowError {
			return "", fmt.Errorf("received empty response from Mojang API")
		}
		return "", nil
	}

	var mojangResp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(body, &mojangResp)
	if err != nil {
		if shouldThrowError {
			return "", fmt.Errorf("error parsing Mojang API response: %v", err)
		}
		return "", nil
	}

	_ = redis.Set(fmt.Sprintf("uuid:%s", strings.ToLower(mojangResp.Name)), mojangResp.ID, 24*60*60)
	_ = redis.Set(fmt.Sprintf("username:%s", mojangResp.ID), mojangResp.Name, 24*60*60)

	return mojangResp.ID, nil
}

func getUsernameFromMojang(uuid string, shouldThrowError bool) (string, error) {
	resp, err := HTTPClient.Get(fmt.Sprintf("https://sessionserver.mojang.com/session/minecraft/profile/%s", uuid))
	if err != nil {
		if shouldThrowError {
			return "", fmt.Errorf("error making request to Mojang API: %v", err)
		}
		return "", nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if shouldThrowError {
			return "", fmt.Errorf("error reading Mojang API response: %v", err)
		}
		return "", nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return "Player not Found", nil
	}

	if len(body) == 0 {
		if shouldThrowError {
			return "", fmt.Errorf("received empty response from Mojang API")
		}
		return "", nil
	}

	var mojangResp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(body, &mojangResp)
	if err != nil {
		if shouldThrowError {
			return "", fmt.Errorf("error parsing Mojang API response: %v", err)
		}
		return "", nil
	}

	_ = redis.Set(fmt.Sprintf("uuid:%s", strings.ToLower(mojangResp.Name)), uuid, 24*60*60)
	_ = redis.Set(fmt.Sprintf("username:%s", uuid), mojangResp.Name, 24*60*60)

	return mojangResp.Name, nil
}

func resolvePlayerFromMojang(uuid string) (*models.MowojangResponse, error) {
	resp, err := HTTPClient.Get(fmt.Sprintf("https://sessionserver.mojang.com/session/minecraft/profile/%s", uuid))
	if err != nil {
		return &models.MowojangResponse{}, fmt.Errorf("error making request to Mojang API: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &models.MowojangResponse{}, fmt.Errorf("error reading Mojang API response: %v", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return &models.MowojangResponse{
			Name: "Player not Found",
			UUID: uuid,
		}, nil
	}

	if len(body) == 0 {
		return &models.MowojangResponse{}, fmt.Errorf("received empty response from Mojang API")
	}

	var mojangResp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(body, &mojangResp)
	if err != nil {
		return &models.MowojangResponse{}, fmt.Errorf("error parsing Mojang API response: %v", err)
	}

	post := &models.MowojangResponse{
		Name: mojangResp.Name,
		UUID: mojangResp.ID,
	}

	mojangBytes, _ := json.Marshal(mojangResp)
	_ = redis.Set(fmt.Sprintf("mowojangUUID:%s", uuid), string(mojangBytes), 24*60*60)
	_ = redis.Set(fmt.Sprintf("mowojangUsername:%s", strings.ToLower(mojangResp.Name)), string(mojangBytes), 24*60*60)
	_ = redis.Set(fmt.Sprintf("uuid:%s", strings.ToLower(mojangResp.Name)), mojangResp.ID, 24*60*60)
	_ = redis.Set(fmt.Sprintf("username:%s", uuid), mojangResp.Name, 24*60*60)

	return post, nil
}
