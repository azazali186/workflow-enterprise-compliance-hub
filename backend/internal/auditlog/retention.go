package auditlog

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/lock"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

// RunRetentionJob periodically hard-deletes audit rows older than the
// retention window so the trail stays complete without unbounded growth. The
// audit log is exactly that — a log — so pruning is a true DELETE, never a
// soft delete. A distributed lock prevents replicas from pruning the same
// rows. days <= 0 disables the job.
func RunRetentionJob(ctx context.Context, db *gorm.DB, l lock.Lock, interval time.Duration, days int, wg *sync.WaitGroup) {
	if interval <= 0 || days <= 0 {
		return
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("audit retention job started", "interval", interval.String(), "retention_days", days)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = lock.WithLock(ctx, l, "jobs:audit-retention", 30*time.Second, func() error {
					cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
					n, err := PruneBefore(ctx, db, cutoff)
					if err != nil {
						slog.Error("audit retention prune failed", "error", err)
						return err
					}
					if n > 0 {
						slog.Info("audit retention pruned rows", "count", n, "older_than", cutoff.Format(time.RFC3339))
					}
					return nil
				})
			}
		}
	}()
}

// pruneBatch bounds each DELETE so a large audit table is trimmed in
// incremental chunks instead of one long transaction that would hold row locks
// on Postgres for minutes.
const pruneBatch = 5000

// PruneBefore hard-deletes audit rows created before the cutoff, in bounded
// batches, and returns how many were removed. GORM-only, no raw SQL.
func PruneBefore(ctx context.Context, db *gorm.DB, cutoff time.Time) (int64, error) {
	var total int64
	for {
		res := db.WithContext(ctx).
			Unscoped().
			Where("created_at < ?", cutoff).
			Limit(pruneBatch).
			Delete(&models.AuditLog{})
		if res.Error != nil {
			return total, res.Error
		}
		if res.RowsAffected == 0 {
			break
		}
		total += res.RowsAffected
	}
	return total, nil
}
