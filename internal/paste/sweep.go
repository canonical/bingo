package paste

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// StartSweep starts a background goroutine that calls repo.DeleteExpired(ctx)
// on every tick of the given interval. It returns a cancel function that stops
// the goroutine and waits for it to exit.
func StartSweep(ctx context.Context, repo Repository, interval time.Duration) func() {
	ctx, cancel := context.WithCancel(ctx)
	ticker := time.NewTicker(interval)
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := repo.DeleteExpired(ctx)
				if err != nil {
					slog.Debug("DeleteExpired error", "error", err)
					continue
				}
				if n > 0 {
					slog.Debug("deleted expired pastes", "count", n)
				}
			}
		}
	}()

	return func() {
		cancel()
		wg.Wait()
	}
}
