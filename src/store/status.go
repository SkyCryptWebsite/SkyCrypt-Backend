package store

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"skycrypt/src/db"

	"github.com/gofiber/fiber/v3"
)

// The store is optional, so the backend runs without it. That makes a missing DATABASE_URL easy to miss, since
// everything else works; these logs say plainly when it is off, why, and what stops working.

const (
	healthCheckEvery    = 30 * time.Second
	unavailableLogEvery = time.Minute
)

var (
	offReason   atomic.Pointer[string]
	lastOffLogs sync.Map // what -> time.Time
)

// Disable records why the store is off and prints it. Prefork children share the parent's reason, so only the
// parent prints, unless the failure is the child's own (childToo).
func Disable(reason string, childToo bool) {
	offReason.Store(&reason)
	if fiber.IsChild() && !childToo {
		return
	}
	fmt.Printf("[STORE] OFF: %s\n", oneLine(reason))
	fmt.Println("[STORE]   Without it: leaderboards answer 503, nothing is served when Hypixel or Mojang fail,")
	fmt.Println("[STORE]   and cache-only API keys get NOT_CACHED for everything not in Redis.")
	fmt.Println("[STORE]   Fix: set DATABASE_URL (see .env.example) to a reachable database and restart.")
}

// Status reports whether the store is on and, if not, why.
func Status() (bool, string) {
	if enabled.Load() {
		return true, ""
	}
	if reason := offReason.Load(); reason != nil {
		return false, *reason
	}
	return false, "the store has not started"
}

// LogUnavailable notes, at most once a minute per kind of request, that something failed because the store is off
// or unreachable.
func LogUnavailable(what string) {
	now := time.Now()
	if last, ok := lastOffLogs.Load(what); ok && now.Sub(last.(time.Time)) < unavailableLogEvery {
		return
	}
	lastOffLogs.Store(what, now)
	_, reason := Status()
	if enabled.Load() {
		reason = "Postgres or Redis didn't answer"
	}
	fmt.Printf("[STORE] %s failed: %s\n", what, oneLine(reason))
}

// oneLine keeps multi-line driver errors on the [STORE] line they belong to.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// logStarted prints what the store runs with, without credentials.
func logStarted(version int) {
	pool := db.Postgres()
	if pool == nil {
		return
	}
	config := pool.Config().ConnConfig
	fmt.Printf("[STORE] on: database %q at %s:%d, schema v%d, %s limit, leaderboards keep the top %d\n",
		config.Database, config.Host, config.Port, version, formatBytes(storageLimit()), BoardSize())
}

// runHealthCheck pings Postgres and logs when it goes away and comes back. Writes fail and reads miss meanwhile;
// nothing else changes, so the backend keeps answering from Hypixel and Redis.
func runHealthCheck() {
	var downSince time.Time
	for {
		time.Sleep(healthCheckEvery)
		pool := db.Postgres()
		if pool == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := pool.Ping(ctx)
		cancel()

		switch {
		case err != nil && downSince.IsZero():
			downSince = time.Now()
			fmt.Printf("[STORE] lost Postgres: %s. Stored data, leaderboards and writes are unavailable until it is back.\n", oneLine(err.Error()))
		case err == nil && !downSince.IsZero():
			fmt.Printf("[STORE] Postgres is back after %v\n", time.Since(downSince).Round(time.Second))
			downSince = time.Time{}
		}
	}
}
