package api

import (
	"context"
	"fmt"
	"io"
	redis "skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/localcache"
	"skycrypt/src/models"
	"skycrypt/src/utility"

	"net/http"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"golang.org/x/sync/singleflight"
)

var resolvePlayerGroup singleflight.Group

const identityCacheTTL = 24 * time.Hour

var (
	uuidLocalCache             = localcache.NewLocalCache[string](8192)
	usernameLocalCache         = localcache.NewLocalCache[string](8192)
	mowojangUUIDLocalCache     = localcache.NewLocalCache[*models.MowojangResponse](8192)
	mowojangUsernameLocalCache = localcache.NewLocalCache[*models.MowojangResponse](8192)
)

func GetUUID(username string, throwAnError ...bool) (string, error) {
	return GetUUIDContext(context.Background(), username, throwAnError...)
}

func GetUUIDContext(ctx context.Context, username string, throwAnError ...bool) (string, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.GetUUID")()
	}

	shouldThrowError := true
	if len(throwAnError) > 0 {
		shouldThrowError = throwAnError[0]
	}

	var post models.MowojangResponse

	key := fmt.Sprintf("uuid:%s", strings.ToLower(username))
	if cachedUUID, ok, _ := uuidLocalCache.Get(key); ok {
		return cachedUUID, nil
	}
	cachedUUID, err := redis.GetContext(ctx, key)
	if err == nil && cachedUUID != "" {
		uuidLocalCache.Set(key, cachedUUID, identityCacheTTL, identityCacheTTL)
		return cachedUUID, nil
	}

	resp, err := getContext(ctx, fmt.Sprintf("https://mowojang.matdoes.dev/%s", username))
	if err != nil || resp.StatusCode != http.StatusOK {
		// Fallback to official Mojang API
		if resp != nil {
			_ = resp.Body.Close()
		}
		return getUUIDFromMojangContext(ctx, username, shouldThrowError)
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
		return getUUIDFromMojangContext(ctx, username, shouldThrowError)
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

	cacheMowojangIdentity(ctx, post.Name, post.UUID, string(body), true)

	return post.UUID, nil
}

func GetUsername(uuid string, throwAnError ...bool) (string, error) {
	return GetUsernameContext(context.Background(), uuid, throwAnError...)
}

func GetUsernameContext(ctx context.Context, uuid string, throwAnError ...bool) (string, error) {
	if utility.IsForensicsEnabled() {
		defer forensics.TrackSpan("api.GetUsername")()
	}

	shouldThrowError := true
	if len(throwAnError) > 0 {
		shouldThrowError = throwAnError[0]
	}

	var post models.MowojangResponse

	key := fmt.Sprintf("username:%s", uuid)
	if cachedUsername, ok, _ := usernameLocalCache.Get(key); ok {
		return cachedUsername, nil
	}
	cachedUsername, err := redis.GetContext(ctx, key)
	if err == nil && cachedUsername != "" {
		usernameLocalCache.Set(key, cachedUsername, identityCacheTTL, identityCacheTTL)
		return cachedUsername, nil
	}

	resp, err := getContext(ctx, fmt.Sprintf("https://mowojang.matdoes.dev/%s", uuid))
	if err != nil || resp.StatusCode != http.StatusOK {
		// Fallback to official Mojang API
		if resp != nil {
			_ = resp.Body.Close()
		}
		return getUsernameFromMojangContext(ctx, uuid, shouldThrowError)
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
		return getUsernameFromMojangContext(ctx, uuid, shouldThrowError)
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

	identityUUID := post.UUID
	if identityUUID == "" {
		identityUUID = uuid
	}
	cacheMowojangIdentity(ctx, post.Name, identityUUID, string(body), true)

	return post.Name, nil
}

func ResolvePlayer(input string, throwAnError ...bool) (*models.MowojangResponse, error) {
	return ResolvePlayerContext(context.Background(), input, throwAnError...)
}

func ResolvePlayerContext(ctx context.Context, input string, throwAnError ...bool) (*models.MowojangResponse, error) {
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
		tempUUID, err := GetUUIDContext(ctx, uuid, shouldThrowError)
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
		return resolvePlayerByUUIDContext(ctx, uuid)
	})
	if err != nil {
		if shouldThrowError {
			return &post, err
		}
		return &post, nil
	}

	return resultIface.(*models.MowojangResponse), nil
}

func ResolvePlayersContext(ctx context.Context, uuids []string) map[string]*models.MowojangResponse {
	resolved := make(map[string]*models.MowojangResponse, len(uuids))
	keys := make([]string, 0, len(uuids))
	keyToUUID := make(map[string]string, len(uuids))

	for _, uuid := range uuids {
		if uuid == "" {
			continue
		}
		if !utility.IsUUID(uuid) {
			mowojang, err := ResolvePlayerContext(ctx, uuid, false)
			if err == nil && mowojang.UUID != "" {
				resolved[mowojang.UUID] = mowojang
			}
			continue
		}
		key := fmt.Sprintf("mowojangUUID:%s", uuid)
		if cached, ok, _ := mowojangUUIDLocalCache.Get(key); ok {
			resolved[uuid] = cached
			continue
		}
		if _, exists := keyToUUID[key]; exists {
			continue
		}
		keys = append(keys, key)
		keyToUUID[key] = uuid
	}

	cache, err := redis.GetManyContext(ctx, keys)
	if err == nil {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		for key, value := range cache {
			var post models.MowojangResponse
			if err := json.Unmarshal([]byte(value), &post); err == nil && post.UUID != "" {
				mowojang := &post
				resolved[keyToUUID[key]] = mowojang
				mowojangUUIDLocalCache.Set(key, mowojang, identityCacheTTL, identityCacheTTL)
			}
		}
	}

	for _, uuid := range uuids {
		if uuid == "" || resolved[uuid] != nil {
			continue
		}
		mowojang, err := resolvePlayerByUUIDFreshContext(ctx, uuid)
		if err == nil && mowojang.UUID != "" {
			resolved[uuid] = mowojang
		}
	}

	return resolved
}

func GetUsernamesContext(ctx context.Context, uuids []string) map[string]string {
	usernames := make(map[string]string, len(uuids))
	keys := make([]string, 0, len(uuids))
	keyToUUID := make(map[string]string, len(uuids))

	for _, uuid := range uuids {
		if uuid == "" {
			continue
		}
		key := fmt.Sprintf("username:%s", uuid)
		if cachedUsername, ok, _ := usernameLocalCache.Get(key); ok {
			usernames[uuid] = cachedUsername
			continue
		}
		if _, exists := keyToUUID[key]; exists {
			continue
		}
		keys = append(keys, key)
		keyToUUID[key] = uuid
	}

	cache, err := redis.GetManyContext(ctx, keys)
	if err == nil {
		for key, value := range cache {
			usernames[keyToUUID[key]] = value
			usernameLocalCache.Set(key, value, identityCacheTTL, identityCacheTTL)
		}
	}

	for _, uuid := range uuids {
		if uuid == "" || usernames[uuid] != "" {
			continue
		}
		username, err := getUsernameFreshContext(ctx, uuid, false)
		if err != nil || username == "" {
			username = "Unknown"
		}
		usernames[uuid] = username
	}

	return usernames
}

func resolvePlayerByUUIDContext(ctx context.Context, uuid string) (*models.MowojangResponse, error) {
	var post models.MowojangResponse

	key := fmt.Sprintf("mowojangUUID:%s", uuid)
	if cached, ok, _ := mowojangUUIDLocalCache.Get(key); ok {
		return cached, nil
	}
	cache, err := redis.GetContext(ctx, key)
	if err == nil && cache != "" {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal([]byte(cache), &post)
		if err == nil {
			mowojangUUIDLocalCache.Set(key, &post, identityCacheTTL, identityCacheTTL)
			return &post, nil
		}
	}

	return resolvePlayerByUUIDFreshContext(ctx, uuid)
}

func resolvePlayerByUUIDFreshContext(ctx context.Context, uuid string) (*models.MowojangResponse, error) {
	var post models.MowojangResponse

	resp, err := getContext(ctx, fmt.Sprintf("https://mowojang.matdoes.dev/%s", uuid))
	if err != nil || resp.StatusCode != http.StatusOK {
		// Fallback to official Mojang API
		if resp != nil {
			_ = resp.Body.Close()
		}
		return resolvePlayerFromMojangContext(ctx, uuid)
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
		return resolvePlayerFromMojangContext(ctx, uuid)
	}

	if len(body) == 0 {
		return &post, fmt.Errorf("received empty response from API")
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(body, &post)
	if err != nil {
		return &post, fmt.Errorf("error parsing JSON: %v", err)
	}

	cacheMowojangIdentity(ctx, post.Name, post.UUID, string(body), true)

	return &post, nil
}

func getUUIDFromMojangContext(ctx context.Context, username string, shouldThrowError bool) (string, error) {
	resp, err := getContext(ctx, fmt.Sprintf("https://api.mojang.com/users/profiles/minecraft/%s", username))
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

	cacheMowojangIdentity(ctx, mojangResp.Name, mojangResp.ID, "", true)

	return mojangResp.ID, nil
}

func getUsernameFromMojangContext(ctx context.Context, uuid string, shouldThrowError bool) (string, error) {
	return getUsernameFreshContext(ctx, uuid, shouldThrowError)
}

func getUsernameFreshContext(ctx context.Context, uuid string, shouldThrowError bool) (string, error) {
	resp, err := getContext(ctx, fmt.Sprintf("https://sessionserver.mojang.com/session/minecraft/profile/%s", uuid))
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

	cacheMowojangIdentity(ctx, mojangResp.Name, uuid, "", true)

	return mojangResp.Name, nil
}

func resolvePlayerFromMojangContext(ctx context.Context, uuid string) (*models.MowojangResponse, error) {
	resp, err := getContext(ctx, fmt.Sprintf("https://sessionserver.mojang.com/session/minecraft/profile/%s", uuid))
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
	cacheMowojangIdentity(ctx, mojangResp.Name, mojangResp.ID, string(mojangBytes), true)

	return post, nil
}

func cacheMowojangIdentity(ctx context.Context, name string, uuid string, body string, includeFullResponse bool) {
	if name == "" || uuid == "" {
		return
	}
	if body == "" && includeFullResponse {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		bodyBytes, err := json.Marshal(models.MowojangResponse{
			UUID: uuid,
			Name: name,
		})
		if err == nil {
			body = string(bodyBytes)
		}
	}

	values := map[string]interface{}{
		fmt.Sprintf("uuid:%s", strings.ToLower(name)): uuid,
		fmt.Sprintf("username:%s", uuid):              name,
	}
	uuidLocalCache.Set(fmt.Sprintf("uuid:%s", strings.ToLower(name)), uuid, identityCacheTTL, identityCacheTTL)
	usernameLocalCache.Set(fmt.Sprintf("username:%s", uuid), name, identityCacheTTL, identityCacheTTL)
	if body != "" {
		values[fmt.Sprintf("mowojang:%s", uuid)] = body
		if includeFullResponse {
			uuidKey := fmt.Sprintf("mowojangUUID:%s", uuid)
			usernameKey := fmt.Sprintf("mowojangUsername:%s", strings.ToLower(name))
			values[uuidKey] = body
			values[usernameKey] = body
			mowojang := &models.MowojangResponse{UUID: uuid, Name: name}
			if err := jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal([]byte(body), mowojang); err != nil {
				mowojang = &models.MowojangResponse{UUID: uuid, Name: name}
			}
			mowojangUUIDLocalCache.Set(uuidKey, mowojang, identityCacheTTL, identityCacheTTL)
			mowojangUsernameLocalCache.Set(usernameKey, mowojang, identityCacheTTL, identityCacheTTL)
		}
	}
	_ = redis.SetManyContext(ctx, values, 24*60*60)
}
