// Package worker contains background jobs.
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/hozer006/backend-challenge/user-management-api/internal/port"
)

// RunUserCounter logs the total number of users every interval until ctx is cancelled.
// It is meant to be started in its own goroutine.
func RunUserCounter(ctx context.Context, counter port.UserCounter, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("user counter stopped")
			return
		case <-ticker.C:
			countCtx, cancel := context.WithTimeout(ctx, interval)
			n, err := counter.Count(countCtx)
			cancel()
			if err != nil {
				logger.Error("count users failed", "error", err)
				continue
			}
			logger.Info("total users", "count", n)
		}
	}
}
