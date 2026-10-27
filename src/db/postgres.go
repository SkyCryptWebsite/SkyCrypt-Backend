package db

import (
	"context"
	"fmt"
	"skycrypt/src/forensics"
	"skycrypt/src/utility"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/jackc/pgx/v5/pgxpool"
)

var pgPool *pgxpool.Pool
var pgMutex sync.RWMutex

// InitPostgres opens the pool. The tables live in src/store, which creates them.
func InitPostgres(url string) error {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return fmt.Errorf("invalid DATABASE_URL: %v", err)
	}
	// Every prefork process opens its own pool, and the instance is shared with the frontend, CMS and Umami.
	// Children only read stored data when Hypixel fails or a key may only read cached data, so they keep few
	// connections and let idle ones go. The parent does all the writing.
	if fiber.IsChild() {
		config.MaxConns = 2
		config.MinConns = 0
		config.MaxConnIdleTime = time.Minute
	} else {
		config.MaxConns = 8
	}

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

	pgMutex.Lock()
	pgPool = pool
	pgMutex.Unlock()

	if !fiber.IsChild() {
		fmt.Print("[POSTGRES] Postgres connected successfully\n")
	}
	return nil
}

// Postgres returns the pool, or nil when Postgres isn't configured.
func Postgres() *pgxpool.Pool {
	pgMutex.RLock()
	defer pgMutex.RUnlock()
	return pgPool
}

func ClosePostgres() {
	if pool := Postgres(); pool != nil {
		pool.Close()
	}
}

func RecordPostgresDependency(ctx context.Context, table, operation string, duration time.Duration, err error) {
	if utility.IsForensicsEnabled() {
		forensics.RecordPostgresDependency(ctx, table, operation, duration, err)
	}
}
