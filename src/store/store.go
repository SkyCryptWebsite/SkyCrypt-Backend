// Package store keeps the last good Hypixel and Mojang data in Postgres. It serves it when Hypixel fails and to API
// keys that may only read cached data, and it is the source for leaderboards.
//
// Profiles are stored once per profile, not once per player who looked them up, with the inventory NBT unpacked so
// zstd (with a dictionary trained on stored data) can compress it. Light rows (players, profiles, members, stats) are
// kept; the compressed blobs are evicted least recently used first to stay under STORAGE_LIMIT_GB.
//
// Prefork children only read. They queue writes in Redis, and the parent process does all the packing, compressing
// and writing, so the work never competes with requests on a child's single thread.
package store

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"skycrypt/src/db"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

var enabled atomic.Bool

// Init prepares the store once Postgres is connected. The parent migrates the schema and starts the background
// workers before any child starts; a child turns the store off if the schema doesn't match this build.
func Init(ctx context.Context) error {
	pool := db.Postgres()
	if pool == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if fiber.IsChild() {
		// The watch turns the store on later if the parent's schema isn't ready yet, and follows schema resets.
		watchOnce.Do(func() { go runChildWatch() })
		version, epoch, err := schemaState(ctx, pool)
		if err != nil {
			return fmt.Errorf("could not read the store schema version: %w", err)
		}
		if want := latestVersion(); version != want {
			return fmt.Errorf("store schema is at version %d, this build needs %d", version, want)
		}
		recordEpoch(epoch)
		enabled.Store(true)
		return nil
	}

	if err := migrate(ctx, pool); err != nil {
		return err
	}
	if err := loadDicts(ctx, pool); err != nil {
		return err
	}
	if _, epoch, err := schemaState(ctx, pool); err == nil {
		recordEpoch(epoch)
	}
	enabled.Store(true)
	logStarted(latestVersion())

	workersOnce.Do(func() {
		go runIngest()
		go runLegacyImport()
		go runEviction()
		go runDictTraining()
		go runArchive()
		warnAboutEviction(ctx)
		go runStatsBackfill()
		go runLeaderboards()
		go runHealthCheck()
	})
	return nil
}

// postgres returns the pool when the store is usable.
func postgres() *pgxpool.Pool {
	if !enabled.Load() {
		return nil
	}
	return db.Postgres()
}
