package api

import (
	"context"
	"fmt"
	"io"
	"os"
	redis "skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/localcache"
	"skycrypt/src/models"
	"skycrypt/src/store"
	"skycrypt/src/utility"
	"sync"

	"net/http"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

var resolvePlayerGroup singleflight.Group

const identityCacheTTL = 24 * time.Hour

// storedNameCacheTTL keeps a name read from the store while Mojang failed: long enough to cover one build and a
// burst of requests, short enough that a renamed player shows up soon.
const storedNameCacheTTL = 5 * time.Minute

var (
	usernameLocalCache = localcache.NewLocalCache[string](8192)
	uuidLocalCache     = localcache.NewLocalCache[string](8192)
	uuidLookupGroup    singleflight.Group
)

// defaultMowojangTimeout bounds one call to the mowojang mirror. It usually answers quickly, but in production it
// sometimes stalled for 1 to 8 s, and every build waiting on a name stalled with it. The official API is the
// fallback.
const defaultMowojangTimeout = 1500 * time.Millisecond

// mowojangTimeout is SKYCRYPT_MOWOJANG_TIMEOUT (such as "800ms"), or the default. It is read on first use rather
// than at package init, which runs before main loads .env.
var mowojangTimeout = sync.OnceValue(func() time.Duration {
	if value := strings.TrimSpace(os.Getenv("SKYCRYPT_MOWOJANG_TIMEOUT")); value != "" {
		if timeout, err := time.ParseDuration(value); err == nil && timeout > 0 {
			return timeout
		}
		fmt.Printf("[MOJANG] invalid SKYCRYPT_MOWOJANG_TIMEOUT %q; using %s\n", value, defaultMowojangTimeout)
	}
	return defaultMowojangTimeout
})

// getMowojang fetches path (a username or UUID) from the mowojang mirror within mowojangTimeout(). ok is false when
// the mirror failed, timed out or doesn't know the player; the caller then asks Mojang.
func getMowojang(ctx context.Context, path string) (body []byte, ok bool) {
	ctx, cancel := context.WithTimeout(ctx, mowojangTimeout())
	defer cancel()

	resp, err := getContext(ctx, "https://mowojang.matdoes.dev/"+path)
	if err != nil {
		return nil, false
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}

	// Read before cancel: the timeout covers the body too.
	body, err = io.ReadAll(resp.Body)
	if err != nil || string(body) == "player not found" {
		return nil, false
	}
	return body, true
}

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

	key := fmt.Sprintf("uuid:%s", strings.ToLower(username))
	if cachedUUID, ok, _ := uuidLocalCache.Get(key); ok {
		return cachedUUID, nil
	}
	cachedUUID, err := redis.GetContext(ctx, key)
	if err == nil && cachedUUID != "" {
		uuidLocalCache.Set(key, cachedUUID, identityCacheTTL, identityCacheTTL)
		return cachedUUID, nil
	}

	// Concurrent lookups of one name share a single Mojang call.
	return Share(ctx, &uuidLookupGroup, fmt.Sprintf("%s:%t", key, shouldThrowError), 15*time.Second, func(ctx context.Context) (string, error) {
		return fetchUUID(ctx, username, shouldThrowError)
	})
}

func fetchUUID(ctx context.Context, username string, shouldThrowError bool) (string, error) {
	var post models.MowojangResponse

	body, ok := getMowojang(ctx, username)
	if !ok {
		// Fallback to official Mojang API
		return getUUIDFromMojangContext(ctx, username, shouldThrowError)
	}

	if len(body) == 0 {
		if shouldThrowError {
			return post.UUID, fmt.Errorf("%w: Mojang sent an empty response", ErrUpstream)
		}
		return "", nil
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(body, &post)
	if err != nil {
		if shouldThrowError {
			return post.UUID, fmt.Errorf("%w: error parsing Mojang JSON: %v", ErrUpstream, err)
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

	key := fmt.Sprintf("username:%s", uuid)
	if cachedUsername, ok, _ := usernameLocalCache.Get(key); ok {
		return cachedUsername, nil
	}
	cachedUsername, err := redis.GetContext(ctx, key)
	if err == nil && cachedUsername != "" {
		usernameLocalCache.Set(key, cachedUsername, identityCacheTTL, identityCacheTTL)
		return cachedUsername, nil
	}

	// The same fetch ResolvePlayer makes, through the same singleflight: a stats build resolves the player, its
	// members and its collection contributors at once, and each used to fetch the same UUID separately.
	resolved, err := resolvePlayerByUUIDShared(ctx, uuid)
	if err != nil {
		if shouldThrowError {
			return "", err
		}
		return "", nil
	}
	return resolved.Name, nil
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
				return &post, fmt.Errorf("error resolving UUID for username '%s': %w", uuid, err)
			}
			return &post, nil
		}
		uuid = tempUUID
	}

	resolved, err := resolvePlayerByUUIDShared(ctx, uuid)
	if err != nil {
		if shouldThrowError {
			return &post, err
		}
		return &post, nil
	}

	return resolved, nil
}

// resolvePlayerByUUIDShared resolves a UUID once however many callers ask for it at the same time (e.g. the handler
// and FormatMembers resolving the same player).
func resolvePlayerByUUIDShared(ctx context.Context, uuid string) (*models.MowojangResponse, error) {
	resultIface, err, _ := resolvePlayerGroup.Do(uuid, func() (interface{}, error) {
		return resolvePlayerByUUIDContext(ctx, uuid)
	})
	if err != nil {
		return nil, err
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
			}
		}
	}

	// Cache misses each need a Mojang call; make them side by side rather than one after another.
	var mu sync.Mutex
	var g errgroup.Group
	g.SetLimit(8)
	// uniqueUnresolvedUUIDs already drops empty, resolved and repeated IDs, so the loop never reads resolved while
	// the goroutines write to it.
	for _, uuid := range uniqueUnresolvedUUIDs(uuids, resolved) {
		g.Go(func() error {
			mowojang, err := ResolvePlayerContext(ctx, uuid, false)
			if err == nil && mowojang.UUID != "" {
				mu.Lock()
				resolved[uuid] = mowojang
				mu.Unlock()
			}
			return nil
		})
	}
	_ = g.Wait()

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

	// Cache misses each need a Mojang call; make them side by side rather than one after another.
	// uniqueUnresolvedUsernames already drops empty, resolved and repeated IDs, so the loop never reads usernames
	// while the goroutines write to it.
	var mu sync.Mutex
	var g errgroup.Group
	g.SetLimit(8)
	for _, uuid := range uniqueUnresolvedUsernames(uuids, usernames) {
		g.Go(func() error {
			username, err := GetUsernameContext(ctx, uuid, false)
			if err != nil || username == "" {
				username = "Unknown"
			}
			mu.Lock()
			usernames[uuid] = username
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	return usernames
}

func uniqueUnresolvedUUIDs(uuids []string, resolved map[string]*models.MowojangResponse) []string {
	seen := make(map[string]struct{}, len(uuids))
	out := make([]string, 0, len(uuids))
	for _, uuid := range uuids {
		if uuid == "" || resolved[uuid] != nil {
			continue
		}
		if _, ok := seen[uuid]; ok {
			continue
		}
		seen[uuid] = struct{}{}
		out = append(out, uuid)
	}
	return out
}

func uniqueUnresolvedUsernames(uuids []string, usernames map[string]string) []string {
	seen := make(map[string]struct{}, len(uuids))
	out := make([]string, 0, len(uuids))
	for _, uuid := range uuids {
		if uuid == "" || usernames[uuid] != "" {
			continue
		}
		if _, ok := seen[uuid]; ok {
			continue
		}
		seen[uuid] = struct{}{}
		out = append(out, uuid)
	}
	return out
}

func resolvePlayerByUUIDContext(ctx context.Context, uuid string) (*models.MowojangResponse, error) {
	var post models.MowojangResponse

	// A UUID and its name are the whole answer, so a name this process already knows needs no Redis or Mojang call.
	if name, ok, _ := usernameLocalCache.Get(fmt.Sprintf("username:%s", uuid)); ok && name != "" {
		return &models.MowojangResponse{UUID: uuid, Name: name}, nil
	}

	key := fmt.Sprintf("mowojangUUID:%s", uuid)
	cache, err := redis.GetContext(ctx, key)
	if err == nil && cache != "" {
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal([]byte(cache), &post)
		if err == nil {
			return &post, nil
		}
	}

	return resolvePlayerByUUIDFreshContext(ctx, uuid)
}

func resolvePlayerByUUIDFreshContext(ctx context.Context, uuid string) (*models.MowojangResponse, error) {
	var post models.MowojangResponse

	body, ok := getMowojang(ctx, uuid)
	if !ok {
		// Fallback to official Mojang API
		return resolvePlayerFromMojangContext(ctx, uuid)
	}

	if len(body) == 0 {
		return &post, fmt.Errorf("%w: Mojang sent an empty response", ErrUpstream)
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(body, &post)
	if err != nil {
		return &post, fmt.Errorf("%w: error parsing Mojang JSON: %v", ErrUpstream, err)
	}

	cacheMowojangIdentity(ctx, post.Name, post.UUID, string(body), true)

	return &post, nil
}

func getUUIDFromMojangContext(ctx context.Context, username string, shouldThrowError bool) (string, error) {
	resp, err := getContext(ctx, fmt.Sprintf("https://api.mojang.com/users/profiles/minecraft/%s", username))
	if err != nil || resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests {
		if resp != nil {
			_ = resp.Body.Close()
		}
		// Both mowojang and Mojang failed: the last known identity is better than none.
		if uuid, _, ok := store.LookupUUID(ctx, username); ok {
			return uuid, nil
		}
		if err == nil {
			err = fmt.Errorf("status code %d", resp.StatusCode)
		}
		if shouldThrowError {
			return "", upstreamError("Mojang", err)
		}
		return "", nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if shouldThrowError {
			return "", upstreamError("Mojang", err)
		}
		return "", nil
	}

	if mojangNotFound(resp.StatusCode) {
		if shouldThrowError {
			return "", fmt.Errorf("%w: invalid username or UUID provided", ErrPlayerNotFound)
		}
		// Not a placeholder name: callers use what this returns as the player's UUID.
		return "", nil
	}

	if len(body) == 0 {
		if shouldThrowError {
			return "", fmt.Errorf("%w: Mojang sent an empty response", ErrUpstream)
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

func resolvePlayerFromMojangContext(ctx context.Context, uuid string) (*models.MowojangResponse, error) {
	resp, err := getContext(ctx, fmt.Sprintf("https://sessionserver.mojang.com/session/minecraft/profile/%s", uuid))
	if err != nil || resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests {
		if resp != nil {
			_ = resp.Body.Close()
		}
		if name, ok := store.LookupName(ctx, uuid); ok {
			// Kept briefly, so the rest of this build and the next few requests don't ask Mojang again while it is
			// failing or rate limiting.
			usernameLocalCache.Set(fmt.Sprintf("username:%s", uuid), name, storedNameCacheTTL, storedNameCacheTTL)
			return &models.MowojangResponse{UUID: uuid, Name: name}, nil
		}
		if err == nil {
			err = fmt.Errorf("status code %d", resp.StatusCode)
		}
		return &models.MowojangResponse{}, upstreamError("Mojang", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &models.MowojangResponse{}, upstreamError("Mojang", err)
	}

	if mojangNotFound(resp.StatusCode) {
		return &models.MowojangResponse{}, fmt.Errorf("%w: no Minecraft player has UUID %s", ErrPlayerNotFound, uuid)
	}

	if len(body) == 0 {
		return &models.MowojangResponse{}, fmt.Errorf("%w: Mojang sent an empty response", ErrUpstream)
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

// mojangNotFound reports a Mojang answer that means the player doesn't exist: 404, 204 (no content) or 400 (not a
// valid name). Without this, such answers parsed into an empty uuid that callers took as a player.
func mojangNotFound(status int) bool {
	return status == http.StatusNotFound || status == http.StatusNoContent || status == http.StatusBadRequest
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
	usernameLocalCache.Set(fmt.Sprintf("username:%s", uuid), name, identityCacheTTL, identityCacheTTL)
	uuidLocalCache.Set(fmt.Sprintf("uuid:%s", strings.ToLower(name)), uuid, identityCacheTTL, identityCacheTTL)
	if body != "" {
		values[fmt.Sprintf("mowojang:%s", uuid)] = body
		if includeFullResponse {
			uuidKey := fmt.Sprintf("mowojangUUID:%s", uuid)
			usernameKey := fmt.Sprintf("mowojangUsername:%s", strings.ToLower(name))
			values[uuidKey] = body
			values[usernameKey] = body
		}
	}
	_ = redis.SetManyContext(ctx, values, 24*60*60)

	store.EnqueueIdentity(ctx, uuid, name)
}
