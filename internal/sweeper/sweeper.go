package sweeper

import (
	"context"
	"log/slog"
	"time"
)

type ExpiryDeleter interface {
	DeleteExpired(ctx context.Context) (int64, error)
}

type Sweeper struct {
	deleter  ExpiryDeleter
	interval time.Duration
	logger   *slog.Logger
}

func New(deleter ExpiryDeleter, interval time.Duration, logger *slog.Logger) *Sweeper {
	return &Sweeper{
		deleter:  deleter,
		interval: interval,
		logger:   logger,
	}
}

func (s *Sweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweep(ctx)
		}
	}
}

func (s *Sweeper) sweep(ctx context.Context) {
	deleted, err := s.deleter.DeleteExpired(ctx)
	if err != nil {
		s.logger.Error("clip sweep failed", slog.Any("error", err))
		return
	}
	if deleted > 0 {
		s.logger.Info("swept expired clips", slog.Int64("deleted", deleted))
	}
}
