package store

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	evictEvery = 10 * time.Minute

	idleBlobAge       = 90 * 24 * time.Hour
	deletedGrace      = 7 * 24 * time.Hour
	orphanPlayerAge   = 180 * 24 * time.Hour
	orphanCheckEvery  = 24 * time.Hour
	evictBatch        = 500
	defaultLimitBytes = 50 << 30

	// Compressed blobs may fill this share of the limit. The rest covers the light tables, indexes, TOAST chunk
	// overhead and dead rows that autovacuum hasn't reused yet.
	blobShare = 0.8
	// Eviction goes this far below the budget so it doesn't run again for every few new blobs.
	evictHeadroom = 0.05
)

// storageLimit is STORAGE_LIMIT_GB (default 50), the most the backend database may use.
func storageLimit() int64 {
	if value, err := strconv.ParseFloat(os.Getenv("STORAGE_LIMIT_GB"), 64); err == nil && value > 0 {
		return int64(value * (1 << 30))
	}
	return defaultLimitBytes
}

func runEviction() {
	var lastOrphanCheck time.Time
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		if pool := postgres(); pool != nil {
			if err := evict(ctx, pool); err != nil {
				fmt.Printf("[STORE] eviction failed: %v\n", err)
			}
			if time.Since(lastOrphanCheck) >= orphanCheckEvery {
				lastOrphanCheck = time.Now()
				if n, err := deleteInBatches(ctx, pool, `DELETE FROM players WHERE ctid = ANY(ARRAY(
					SELECT p.ctid FROM players p WHERE p.touched_at < now() - make_interval(secs => $2)
						AND NOT EXISTS (SELECT 1 FROM profile_members m WHERE m.player_uuid = p.uuid)
					LIMIT $1)) RETURNING 0`, orphanPlayerAge.Seconds()); err != nil {
					fmt.Printf("[STORE] could not expire players: %v\n", err)
				} else if n.rows > 0 {
					fmt.Printf("[STORE] expired %d players\n", n.rows)
				}
			}
		}
		cancel()
		time.Sleep(evictEvery)
	}
}

func evict(ctx context.Context, pool *pgxpool.Pool) error {
	// Deleted profiles are in nobody's list anymore, so their blobs can't be served.
	if n, err := deleteInBatches(ctx, pool, `DELETE FROM blobs WHERE ctid = ANY(ARRAY(
		SELECT b.ctid FROM blobs b JOIN profiles p USING (profile_id) WHERE p.deleted_at IS NOT NULL LIMIT $1)) RETURNING bytes`); err != nil {
		return err
	} else if n.rows > 0 {
		fmt.Printf("[STORE] dropped %d blobs (%s) of deleted profiles\n", n.rows, formatBytes(n.bytes))
	}

	// After the grace period the light rows go too; members and stats cascade.
	if n, err := deleteInBatches(ctx, pool, `DELETE FROM profiles WHERE ctid = ANY(ARRAY(
		SELECT ctid FROM profiles WHERE deleted_at < now() - make_interval(secs => $2) LIMIT $1)) RETURNING 0`,
		deletedGrace.Seconds()); err != nil {
		return err
	} else if n.rows > 0 {
		fmt.Printf("[STORE] purged %d deleted profiles\n", n.rows)
	}

	if n, err := deleteInBatches(ctx, pool, `DELETE FROM blobs WHERE ctid = ANY(ARRAY(
		SELECT ctid FROM blobs WHERE accessed_on < current_date - $2::integer LIMIT $1)) RETURNING bytes`,
		int(idleBlobAge.Hours()/24)); err != nil {
		return err
	} else if n.rows > 0 {
		fmt.Printf("[STORE] wiped %d idle blobs (%s)\n", n.rows, formatBytes(n.bytes))
	}

	return evictToBudget(ctx, pool)
}

// evictToBudget deletes the least recently used blobs while they take more than their share of the limit.
//
// It budgets the bytes stored in blobs, not the database size: Postgres reuses the space of deleted rows instead of
// giving it back, so the database size barely drops after an eviction and steering by it would evict everything.
func evictToBudget(ctx context.Context, pool *pgxpool.Pool) error {
	limit := storageLimit()

	var stored, light, database int64
	err := pool.QueryRow(ctx, `SELECT
		(SELECT coalesce(sum(bytes), 0) FROM blobs),
		pg_total_relation_size('players') + pg_total_relation_size('profiles') + pg_total_relation_size('profile_members')
			+ pg_total_relation_size('member_stats') + pg_total_relation_size('resources') + pg_total_relation_size('codec_dicts'),
		pg_database_size(current_database())`).Scan(&stored, &light, &database)
	if err != nil {
		return err
	}

	if database > limit*95/100 {
		fmt.Printf("[STORE] database is %s of a %s limit; blobs hold %s, light tables %s\n",
			formatBytes(database), formatBytes(limit), formatBytes(stored), formatBytes(light))
	}

	budget := int64(float64(limit)*blobShare) - light
	if stored <= budget {
		return nil
	}

	target := stored - int64(float64(budget)*(1-evictHeadroom))
	var freed, rows int64
	for freed < target {
		// From the oldest batch, only as many rows as it takes to free the rest of the target.
		n, err := deleteBatch(ctx, pool, `DELETE FROM blobs WHERE ctid = ANY(ARRAY(
			SELECT ctid FROM (
				SELECT ctid, sum(bytes) OVER (ORDER BY accessed_on, profile_id, kind) - bytes AS before
				FROM (SELECT ctid, bytes, accessed_on, profile_id, kind FROM blobs ORDER BY accessed_on LIMIT $1) oldest
			) running WHERE before < $2)) RETURNING bytes`, target-freed)
		if err != nil {
			return err
		}
		if n.rows == 0 {
			break
		}
		freed += n.bytes
		rows += n.rows
	}
	fmt.Printf("[STORE] evicted %d least recently used blobs (%s) to stay under %s\n", rows, formatBytes(freed), formatBytes(limit))
	return nil
}

type deleted struct {
	rows  int64
	bytes int64
}

// deleteInBatches repeats a DELETE ... LIMIT $1 RETURNING <bytes> until it deletes nothing, so no single statement
// holds locks for long. Extra arguments follow the batch size.
func deleteInBatches(ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) (deleted, error) {
	var total deleted
	for {
		n, err := deleteBatch(ctx, pool, sql, args...)
		if err != nil {
			return total, err
		}
		total.rows += n.rows
		total.bytes += n.bytes
		if n.rows < evictBatch {
			return total, nil
		}
	}
}

func deleteBatch(ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) (deleted, error) {
	rows, err := pool.Query(ctx, sql, append([]any{evictBatch}, args...)...)
	if err != nil {
		return deleted{}, err
	}
	defer rows.Close()

	var n deleted
	for rows.Next() {
		var size int64
		if err := rows.Scan(&size); err != nil {
			return n, err
		}
		n.rows++
		n.bytes += size
	}
	return n, rows.Err()
}

func formatBytes(n int64) string {
	if n < 1<<30 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
}
