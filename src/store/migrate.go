package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrationLock is a fixed pg_advisory_lock key, so two parents starting at once don't both migrate.
const migrationLock = 0x736b7963 // "skyc"

type migration struct {
	version int
	sql     string
}

func migrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}

	var out []migration
	for _, entry := range entries {
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		version, err := strconv.Atoi(prefix)
		if !ok || err != nil {
			return nil, fmt.Errorf("migration %s needs a numeric prefix", entry.Name())
		}
		sql, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, migration{version: version, sql: string(sql)})
	}
	slices.SortFunc(out, func(a, b migration) int { return a.version - b.version })
	return out, nil
}

// latestVersion is the schema version this build expects.
func latestVersion() int {
	all, err := migrations()
	if err != nil || len(all) == 0 {
		return 0
	}
	return all[len(all)-1].version
}

// migrate applies every migration newer than the database, each in its own transaction.
func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	all, err := migrations()
	if err != nil {
		return err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLock); err != nil {
		return err
	}
	defer func() { _, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationLock) }()

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    integer PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return err
	}

	current, err := schemaVersion(ctx, conn.Conn())
	if err != nil {
		return err
	}

	for _, m := range all {
		if m.version <= current {
			continue
		}
		err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, m.sql); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, m.version)
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %d: %w", m.version, err)
		}
		fmt.Printf("[POSTGRES] applied migration %d\n", m.version)
	}
	return nil
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func schemaVersion(ctx context.Context, db queryRower) (int, error) {
	var version int
	err := db.QueryRow(ctx, `SELECT coalesce(max(version), 0) FROM schema_migrations`).Scan(&version)
	return version, err
}
