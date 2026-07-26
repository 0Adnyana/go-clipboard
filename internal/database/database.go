package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const probeTimeout = 3 * time.Second

type Pool struct {
	*pgxpool.Pool
}

func NewPool(ctx context.Context, databaseURL string) (*Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	return &Pool{Pool: pool}, nil
}

func (p *Pool) VerifyConnectivity(ctx context.Context, logger *slog.Logger) {
	if err := p.Pool.Ping(ctx); err != nil {
		logger.Error("database connectivity check failed", slog.Any("error", err))
		return
	}
	logger.Info("database connectivity verified")
}

type ProbeResult struct {
	Reachable bool
	Latency   time.Duration
	Err       error
}

func (p *Pool) Probe(ctx context.Context) ProbeResult {
	if p == nil || p.Pool == nil {
		return ProbeResult{Reachable: false, Err: fmt.Errorf("database pool is not configured")}
	}

	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	start := time.Now()
	err := p.Pool.Ping(probeCtx)
	latency := time.Since(start)
	if err != nil {
		return ProbeResult{Reachable: false, Latency: latency, Err: err}
	}
	return ProbeResult{Reachable: true, Latency: latency}
}
