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
	if err != nil {
		if shouldThrowError {
			return post.UUID, fmt.Errorf("error making request: %v", err)
		}
		return "", nil
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

	if resp.StatusCode == http.StatusNotFound || string(body) == "player not found" {
		if shouldThrowError {
			return post.UUID, fmt.Errorf("invalid username or UUID provided")
		}

		return "Player not Found", nil
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
	if err != nil {
		if shouldThrowError {
			return post.Name, fmt.Errorf("error making request: %v", err)
		}
		return "", nil
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

	if resp.StatusCode == http.StatusNotFound || string(body) == "player not found" {
		return "Player not Found", nil
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
	if err != nil {
		return &post, fmt.Errorf("error making request: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &post, fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode == http.StatusNotFound || string(body) == "player not found" {
		return &models.MowojangResponse{
			Name: "Player not Found",
			UUID: uuid,
		}, nil
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
