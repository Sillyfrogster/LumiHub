package delivery

import (
	"context"
	"fmt"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/db"
)

// RunSweeper clears deliveries past their retention window, so a machine that never returns leaves nothing.
func (s *Service) RunSweeper(ctx context.Context, onError func(error)) {
	ticker := time.NewTicker(s.settings.SweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.Sweep(ctx); err != nil && ctx.Err() == nil && onError != nil {
				onError(err)
			}
		}
	}
}

// Sweep deletes one batch of expired deliveries and reports how many went.
func (s *Service) Sweep(ctx context.Context) (int64, error) {
	swept, err := db.New(s.pool).DeleteExpiredDeliveries(ctx, sweepBatch)
	if err != nil {
		return 0, fmt.Errorf("sweep expired deliveries: %w", err)
	}
	return swept, nil
}

const sweepBatch = 200
