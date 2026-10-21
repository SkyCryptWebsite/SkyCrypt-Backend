package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"skycrypt/src/forensics"
	"skycrypt/src/utility"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// The fallback only has to be as fresh as the last time it saved anyone from an outage.
	fallbackWriteInterval = time.Hour
	fallbackRetention     = 30 * 24 * time.Hour
	fallbackWriteTimeout  = 5 * time.Second
)

var pgPool *pgxpool.Pool
var pgMutex sync.RWMutex

// Bounds in-flight fallback writes so a slow Postgres drops writes instead of piling up goroutines.
var fallbackWriteSlots = make(chan struct{}, 32)

const schema = `
CREATE TABLE IF NOT EXISTS fallback_cache (
	key        text PRIMARY KEY,
	body       bytea NOT NULL,
	hash       bigint NOT NULL,
	fetched_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE fallback_cache ALTER COLUMN body SET STORAGE EXTERNAL;
ALTER TABLE fallback_cache SET (autovacuum_vacuum_scale_factor = 0.02);
CREATE INDEX IF NOT EXISTS fallback_cache_fetched_at ON fallback_cache (fetched_at);
`

func InitPostgres(url string) error {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return fmt.Errorf("invalid DATABASE_URL: %v", err)
	}
	// Every prefork process opens its own pool, and the instance is shared with the frontend, CMS and Umami.
	config.MaxConns = 4

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("could not connect to Postgres: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("could not ping Postgres: %v", err)
	}

	// The parent process runs before the prefork children start, so the schema exists by the time they connect.
	if os.Getenv("FIBER_PREFORK_CHILD") == "" {
		if _, err := pool.Exec(ctx, schema); err != nil {
			pool.Close()
			return fmt.Errorf("could not create Postgres schema: %v", err)
		}
		go pruneFallbackLoop()
		fmt.Print("[POSTGRES] Postgres connected successfully\n")
	}

	pgMutex.Lock()
	pgPool = pool
	pgMutex.Unlock()

	return nil
}

func getPostgres() *pgxpool.Pool {
	pgMutex.RLock()
	defer pgMutex.RUnlock()
	return pgPool
}

func ClosePostgres() {
	if pool := getPostgres(); pool != nil {
		pool.Close()
	}
}

// SaveFallback stores the last good upstream response for key in the background.
// Rows are rewritten at most once per fallbackWriteInterval, and an unchanged body keeps its stored copy.
func SaveFallback(key string, body []byte) {
	pool := getPostgres()
	if pool == nil || len(body) == 0 {
		return
	}

	select {
	case fallbackWriteSlots <- struct{}{}:
	default:
		return
	}

	go func() {
		defer func() { <-fallbackWriteSlots }()

		ctx, cancel := context.WithTimeout(context.Background(), fallbackWriteTimeout)
		defer cancel()

		start := time.Now()
		_, err := pool.Exec(ctx, `
			INSERT INTO fallback_cache (key, body, hash) VALUES ($1, $2, $3)
			ON CONFLICT (key) DO UPDATE SET
				body = CASE WHEN fallback_cache.hash = excluded.hash THEN fallback_cache.body ELSE excluded.body END,
				hash = excluded.hash,
				fetched_at = now()
			WHERE fallback_cache.fetched_at < now() - make_interval(secs => $4)`,
			key, zstdEncoder.EncodeAll(body, nil), int64(xxhash.Sum64(body)), fallbackWriteInterval.Seconds())
		recordPostgresDependency(ctx, "fallback_cache", "upsert", time.Since(start), err)
	}()
}

// GetFallback returns the last good upstream response stored for key.
func GetFallback(ctx context.Context, key string) ([]byte, bool) {
	pool := getPostgres()
	if pool == nil {
		return nil, false
	}

	start := time.Now()
	var compressed []byte
	err := pool.QueryRow(ctx, `SELECT body FROM fallback_cache WHERE key = $1`, key).Scan(&compressed)
	if errors.Is(err, pgx.ErrNoRows) {
		recordPostgresDependency(ctx, "fallback_cache", "select", time.Since(start), nil)
		return nil, false
	}
	recordPostgresDependency(ctx, "fallback_cache", "select", time.Since(start), err)
	if err != nil {
		return nil, false
	}

	body, err := zstdDecoder.DecodeAll(compressed, nil)
	if err != nil {
		return nil, false
	}
	return body, true
}

func pruneFallbackLoop() {
	for {
		if pool := getPostgres(); pool != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			tag, err := pool.Exec(ctx, `DELETE FROM fallback_cache WHERE fetched_at < now() - make_interval(secs => $1)`, fallbackRetention.Seconds())
			cancel()
			if err != nil {
				fmt.Printf("[POSTGRES] could not prune fallback cache: %v\n", err)
			} else if tag.RowsAffected() > 0 {
				fmt.Printf("[POSTGRES] pruned %d fallback cache rows\n", tag.RowsAffected())
			}
		}
		time.Sleep(24 * time.Hour)
	}
}

func recordPostgresDependency(ctx context.Context, table, operation string, duration time.Duration, err error) {
	if utility.IsForensicsEnabled() {
		forensics.RecordPostgresDependency(ctx, table, operation, duration, err)
	}
}
