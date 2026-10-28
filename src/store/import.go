package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The old store was one key/value table, fallback_cache(key, body zstd, hash, fetched_at), using the Redis key
// names. importLegacy moves its rows into the new tables, newest first so the players most likely to be looked up
// again come over first, and drops the table once it is empty.
//
// Each batch is deleted as it is claimed, so a restart picks up where the last run stopped. A batch that fails half
// way loses a few rows; they are cache data and come back on the next lookup.
const (
	importBatch   = 50
	importWorkers = 2
	// After this many profiles the dictionaries are trained, so the rest of the import is written with them.
	importTrainAfter = firstDictMinBlobs
)

func runLegacyImport() {
	pool := postgres()
	if pool == nil {
		return
	}
	if err := importLegacy(context.Background(), pool); err != nil {
		fmt.Printf("[STORE] importing fallback_cache stopped: %v\n", err)
	}
}

func importLegacy(ctx context.Context, pool *pgxpool.Pool) error {
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('fallback_cache') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return nil
	}

	var remaining int64
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM fallback_cache`).Scan(&remaining)
	fmt.Printf("[STORE] importing %d rows from fallback_cache\n", remaining)

	start := time.Now()
	var imported, failed, profiles atomic.Int64
	var trainOnce sync.Once
	var wg sync.WaitGroup
	errs := make(chan error, importWorkers)

	for range importWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				rows, err := claimLegacyRows(ctx, pool)
				if err != nil {
					errs <- err
					return
				}
				if len(rows) == 0 {
					return
				}
				for _, row := range rows {
					isProfiles := strings.HasPrefix(row.key, "profiles:")
					if err := withRetry(func() error { return importLegacyRow(ctx, row) }); err != nil {
						failed.Add(1)
					} else if isProfiles {
						profiles.Add(1)
					}
					if n := imported.Add(1); n%5000 == 0 {
						fmt.Printf("[STORE] imported %d/%d fallback_cache rows in %v\n", n, remaining, time.Since(start).Round(time.Second))
					}
				}
				if profiles.Load() >= importTrainAfter {
					trainOnce.Do(func() {
						for _, kind := range dictKinds {
							if err := maybeTrainDict(ctx, kind); err != nil {
								fmt.Printf("[STORE] could not train a dictionary for kind %d: %v\n", kind, err)
							}
						}
					})
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	if err := <-errs; err != nil {
		return err
	}

	// Dropping the table is what gives its disk space back.
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS fallback_cache`); err != nil {
		return err
	}
	fmt.Printf("[STORE] imported fallback_cache in %v: %d rows, %d skipped; table dropped\n",
		time.Since(start).Round(time.Second), imported.Load(), failed.Load())
	return nil
}

type legacyRow struct {
	key       string
	body      []byte
	fetchedAt time.Time
}

func claimLegacyRows(ctx context.Context, pool *pgxpool.Pool) ([]legacyRow, error) {
	rows, err := pool.Query(ctx, `DELETE FROM fallback_cache WHERE key IN (
			SELECT key FROM fallback_cache ORDER BY fetched_at DESC LIMIT $1 FOR UPDATE SKIP LOCKED)
		RETURNING key, body, fetched_at`, importBatch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []legacyRow
	for rows.Next() {
		var row legacyRow
		if err := rows.Scan(&row.key, &row.body, &row.fetchedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func importLegacyRow(ctx context.Context, row legacyRow) error {
	body, err := plainDecoder.DecodeAll(row.body, nil)
	if err != nil {
		return err
	}

	kind, id, _ := strings.Cut(row.key, ":")
	switch kind {
	case "profiles":
		return storeProfiles(ctx, id, body, row.fetchedAt)
	case "player":
		return storePlayer(ctx, id, body, row.fetchedAt)
	case "museum":
		if !successful(body) {
			return nil
		}
		return storeBlob(ctx, id, kindMuseum, body, row.fetchedAt)
	case "garden":
		if !successful(body) {
			return nil
		}
		return storeBlob(ctx, id, kindGarden, body, row.fetchedAt)
	case "username":
		return storeIdentity(ctx, id, string(body), row.fetchedAt)
	case "mowojangUUID":
		var identity struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(body, &identity); err != nil {
			return err
		}
		return storeIdentity(ctx, identity.ID, identity.Name, row.fetchedAt)
	case "uuid":
		// Only the lowercased name: username:<uuid> carries the real one.
		return nil
	case "skyblock_items", "skyblock_collections":
		_, data := compress(0, body)
		_, err := postgres().Exec(ctx, `INSERT INTO resources (name, fetched_at, data) VALUES ($1, $2, $3)
			ON CONFLICT (name) DO UPDATE SET fetched_at = excluded.fetched_at, data = excluded.data
			WHERE resources.fetched_at < excluded.fetched_at`, kind, row.fetchedAt, data)
		return err
	}
	return errors.New("unknown fallback_cache key " + row.key)
}
