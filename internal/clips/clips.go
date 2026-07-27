package clips

import (
	"errors"
	"time"
)

const (
	MaxBodyBytes = 262144 // 256 KiB
	Lease        = 2 * time.Hour
)

var (
	ErrSlugInUse    = errors.New("slug in use")
	ErrNotFound     = errors.New("clip not found")
	ErrBodyTooLarge = errors.New("body too large")
)

// Clip is a stored paste with a fixed anonymous lease.
type Clip struct {
	Slug      string
	Body      string
	CreatedAt time.Time
	ExpiresAt time.Time
}
