package clips

import (
	"context"
	"time"
)

// Store persists clip claims and live reads.
type Store interface {
	ClaimClip(ctx context.Context, slug, body string, createdAt, expiresAt time.Time) (*Clip, error)
	GetLiveClip(ctx context.Context, slug string) (*Clip, error)
}
