package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"skycrypt/src/db"
	"skycrypt/src/security"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The store heals itself instead of staying off until a restart:
//   - Postgres unreachable at boot: Start keeps retrying in the background.
//   - Database dropped while running: the parent creates it again (with CREATEDB).
//   - Tables dropped while running: the parent runs the migrations again.
//
// After the schema is recreated, numbered things cached in every process (stat key ids, dictionary ids, board
// thresholds) mean something else, so each process drops them. Processes notice through the schema epoch: the time
// migration 1 was applied, which changes whenever the schema is created from scratch.

const (
	retryStartEvery = 30 * time.Second
	recoverTimeout  = 5 * time.Minute
)

var (
	databaseURL  string
	workersOnce  sync.Once
	watchOnce    sync.Once
	checkSchema  = make(chan struct{}, 1)
	schemaEpoch  atomic.Pointer[time.Time]
	recoverMutex sync.Mutex
)

// Start connects to Postgres and starts the store. When that fails, the store is off until a background retry
// succeeds.
func Start(url string) {
	databaseURL = url
	if err := start(); err != nil {
		// Only the parent prints: children fail the same way (or wait on the parent's schema), and their watch turns
		// them on once the parent's store is up.
		Disable(err.Error(), false)
		go retryStart()
	}
}

func start() error {
	if db.Postgres() == nil {
		if err := db.InitPostgres(databaseURL); err != nil {
			return fmt.Errorf("could not connect to Postgres: %v", security.RedactError(err))
		}
	}
	if err := Init(context.Background()); err != nil {
		return fmt.Errorf("could not start the store: %v%s", err, permissionHint(err))
	}
	return nil
}

// permissionHint says how to fix the database user's rights when it may connect but not create tables, which is
// the default for non-owners since Postgres 15.
func permissionHint(err error) string {
	var pgErr *pgconn.PgError
	pool := db.Postgres()
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" || pool == nil {
		return ""
	}
	config := pool.Config().ConnConfig
	return fmt.Sprintf(". The user %q may not create tables in %q; as a superuser, run: ALTER DATABASE %q OWNER TO %q;"+
		" The store retries every %v, so it starts once that is done", config.User, config.Database, config.Database,
		config.User, retryStartEvery)
}

func retryStart() {
	for {
		time.Sleep(retryStartEvery)
		if err := start(); err == nil {
			if !fiber.IsChild() {
				fmt.Println("[STORE] started after retrying")
			}
			return
		}
	}
}

// requestSchemaCheck asks the parent's health check to look at the schema now, for example after a write found a
// table missing.
func requestSchemaCheck() {
	select {
	case checkSchema <- struct{}{}:
	default:
	}
}

// isMissingSchema reports an error from a table or database that doesn't exist.
func isMissingSchema(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "42P01" || pgErr.Code == "3D000")
}

// schemaState returns the schema version and epoch. A missing schema_migrations table is version 0.
func schemaState(ctx context.Context, pool *pgxpool.Pool) (int, time.Time, error) {
	var version int
	var epoch *time.Time
	err := pool.QueryRow(ctx, `SELECT coalesce(max(version), 0), min(applied_at) FROM schema_migrations`).Scan(&version, &epoch)
	if isMissingSchema(err) {
		return 0, time.Time{}, nil
	}
	if err != nil {
		return 0, time.Time{}, err
	}
	if epoch == nil {
		return version, time.Time{}, nil
	}
	return version, *epoch, nil
}

func recordEpoch(epoch time.Time) {
	schemaEpoch.Store(&epoch)
}

func epochChanged(epoch time.Time) bool {
	current := schemaEpoch.Load()
	return current == nil || !current.Equal(epoch)
}

// resetCaches forgets everything that refers to rows of a schema that no longer exists.
func resetCaches() {
	statKeyCache.Lock()
	statKeyCache.byKey, statKeyCache.byID = map[string]int32{}, map[int32]string{}
	statKeyCache.Unlock()
	thresholds.Clear()
	kindEncoders.Clear()
	dictDecoders.Store(nil)
}

// ensureSchema migrates again when the schema is gone or behind, and resets caches when it was recreated by
// another process. Parent only.
func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	recoverMutex.Lock()
	defer recoverMutex.Unlock()

	version, epoch, err := schemaState(ctx, pool)
	if err != nil {
		return err
	}
	want := latestVersion()
	if version >= want {
		if epochChanged(epoch) {
			resetCaches()
			recordEpoch(epoch)
			if err := loadDicts(ctx, pool); err != nil {
				return err
			}
		}
		return nil
	}

	if version == 0 {
		fmt.Println("[STORE] the store's tables are gone; creating them again")
	} else {
		fmt.Printf("[STORE] the schema is at version %d, this build needs %d; migrating\n", version, want)
	}
	if err := migrate(ctx, pool); err != nil {
		return err
	}
	resetCaches()
	if err := loadDicts(ctx, pool); err != nil {
		return err
	}
	if _, epoch, err := schemaState(ctx, pool); err == nil {
		recordEpoch(epoch)
	}

	// The boards point at rows that are gone. Rebuild them from what is stored now.
	if client, err := db.Redis(); err == nil {
		_ = client.Del(ctx, readyKey).Err()
	}
	go func() {
		rebuildCtx, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		if err := rebuildBoards(rebuildCtx); err != nil && !errors.Is(err, errNoRebuildLock) {
			fmt.Printf("[STORE] could not rebuild the leaderboards: %v\n", err)
		}
	}()
	fmt.Printf("[STORE] schema v%d is back; data starts filling again as players are viewed\n", want)
	return nil
}

// runChildWatch keeps a child's view of the store current: it turns the store on once the parent's schema is ready
// and drops cached ids when the schema was recreated.
func runChildWatch() {
	for {
		time.Sleep(healthCheckEvery)
		pool := db.Postgres()
		if pool == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		version, epoch, err := schemaState(ctx, pool)
		cancel()
		if err != nil || version != latestVersion() {
			continue
		}
		if epochChanged(epoch) {
			resetCaches()
			recordEpoch(epoch)
		}
		enabled.Store(true)
	}
}
