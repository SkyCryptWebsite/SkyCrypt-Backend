package routes

import (
	"context"
	"fmt"
	"skycrypt/src/db"
	"time"
)

const selectedProfileTTLSeconds = 5 * 60

func selectedProfileCacheKey(uuid string) string {
	return fmt.Sprintf("selected_profile:%s", uuid)
}

func getCachedSelectedProfileID(ctx context.Context, uuid string) string {
	key := selectedProfileCacheKey(uuid)
	profileID, err := db.GetContext(ctx, key)
	if err != nil {
		return ""
	}
	return profileID
}

func cacheSelectedProfileID(ctx context.Context, uuid string, profileID string) {
	if uuid == "" || profileID == "" {
		return
	}

	key := selectedProfileCacheKey(uuid)
	cacheCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = db.SetContext(cacheCtx, key, profileID, selectedProfileTTLSeconds)
}
