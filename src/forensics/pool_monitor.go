package forensics

import (
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type PoolMonitor struct {
	logger *zap.Logger
}

func NewPoolMonitor() *PoolMonitor {
	return &PoolMonitor{logger: Logger}
}

func (pm *PoolMonitor) MonitorRedisPool(client *redis.Client) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := client.PoolStats()

		pm.logger.Info("redis_pool_stats",
			zap.Uint32("hits", stats.Hits),
			zap.Uint32("misses", stats.Misses),
			zap.Uint32("timeouts", stats.Timeouts),
			zap.Uint32("total_conns", stats.TotalConns),
			zap.Uint32("idle_conns", stats.IdleConns),
			zap.Uint32("stale_conns", stats.StaleConns),
		)

		if stats.Timeouts > 0 {
			pm.logger.Error("redis_pool_exhaustion",
				zap.Uint32("timeouts", stats.Timeouts),
				zap.Uint32("total_conns", stats.TotalConns),
				zap.String("action_required", "Increase Redis pool size or reduce concurrent requests"),
			)
		}

		totalOps := stats.Hits + stats.Misses
		if totalOps > 0 {
			hitRate := float64(stats.Hits) / float64(totalOps) * 100

			if hitRate < 90 {
				pm.logger.Warn("redis_pool_low_hit_rate",
					zap.Float64("hit_rate_percent", hitRate),
					zap.String("suggestion", "Connection pool hit rate low. Increase min idle connections"),
				)
			}
		}

		if stats.StaleConns > 0 {
			pm.logger.Warn("redis_stale_connections",
				zap.Uint32("stale_conns", stats.StaleConns),
				zap.String("suggestion", "Stale connections detected. Review idle timeout settings"),
			)
		}
	}
}

func (pm *PoolMonitor) MongoPoolCheck() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		pm.logger.Info("mongo_connection_pool_check",
			zap.String("status", "monitoring"),
			zap.String("note", "MongoDB Go driver does not expose pool stats. Check mongod logs or use db.serverStatus().connections"),
		)
	}
}
