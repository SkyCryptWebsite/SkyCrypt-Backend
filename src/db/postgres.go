package db

import (
	"context"
	"errors"
	"fmt"
	"skycrypt/src/forensics"
	"skycrypt/src/utility"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	err = pool.Ping(ctx)
	// The parent creates a missing database (dropped, or never created) instead of running without one.
	if IsMissingDatabase(err) && !fiber.IsChild() {
		if _, createErr := EnsureDatabase(ctx, url); createErr != nil {
			err = fmt.Errorf("%w; creating it failed: %v", err, createErr)
		} else {
			err = pool.Ping(ctx)
		}
	}
	if err != nil {
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

// IsMissingDatabase reports a connection refused because its database doesn't exist (SQLSTATE 3D000).
func IsMissingDatabase(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "3D000"
}

// EnsureDatabase creates the database DATABASE_URL names when it doesn't exist, through the server's "postgres"
// maintenance database. The user needs the CREATEDB privilege (or to be a superuser). It reports whether it created
// the database.
func EnsureDatabase(ctx context.Context, url string) (bool, error) {
	config, err := pgx.ParseConfig(url)
	if err != nil {
		return false, err
	}
	name := config.Database
	if name == "" || name == "postgres" {
		return false, errors.New("DATABASE_URL names no database to create")
	}
	config.Database = "postgres"

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return false, err
	}
	defer func() { _ = conn.Close(context.Background()) }()

	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	if _, err := conn.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize()); err != nil {
		return false, err
	}
	fmt.Printf("[POSTGRES] created the missing database %q\n", name)
	return true, nil
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
