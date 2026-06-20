package routes

import (
	"context"
	"fmt"
	"skycrypt/src/db"
	"skycrypt/src/localcache"
	"time"
)

const selectedProfileTTLSeconds = 5 * 60

var selectedProfileCache = localcache.NewLocalCache[string](512)

func selectedProfileCacheKey(uuid string) string {
	return fmt.Sprintf("selected_profile:%s", uuid)
}

func getCachedSelectedProfileID(ctx context.Context, uuid string) string {
	key := selectedProfileCacheKey(uuid)
	if profileID, ok, _ := selectedProfileCache.Get(key); ok {
		return profileID
	}

	profileID, err := db.GetContext(ctx, key)
	if err != nil || profileID == "" {
		return ""
	}
	selectedProfileCache.Set(key, profileID, selectedProfileTTLSeconds*time.Second, selectedProfileTTLSeconds*time.Second)
	return profileID
}

func cacheSelectedProfileID(ctx context.Context, uuid string, profileID string) {
	if uuid == "" || profileID == "" {
		return
	}

	key := selectedProfileCacheKey(uuid)
	selectedProfileCache.Set(key, profileID, selectedProfileTTLSeconds*time.Second, selectedProfileTTLSeconds*time.Second)
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = db.SetContext(cacheCtx, key, profileID, selectedProfileTTLSeconds)
	}()
}
