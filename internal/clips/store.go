package clips

import (
	"context"
	"time"
)

// Store persists clip claims, live reads, sweeps, and availability lookups.
type Store interface {
	ClaimClip(ctx context.Context, slug, body string, createdAt, expiresAt time.Time) (*Clip, error)
	GetLiveClip(ctx context.Context, slug string) (*Clip, error)
	DeleteExpiredClips(ctx context.Context) (int64, error)
	SlugIsLive(ctx context.Context, slug string) (bool, error)
}
