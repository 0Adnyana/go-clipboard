package clips

import (
	"errors"
	"math"
	"time"
)

const (
	MaxBodyBytes = 262144 // 256 KiB
	AnonCeiling  = 2 * time.Hour
)

var (
	Preset10m = 10 * time.Minute
	Preset1h  = 1 * time.Hour
	Preset2h  = 2 * time.Hour

	Presets = []time.Duration{Preset10m, Preset1h, Preset2h}

	DefaultPreset = Preset2h
)

var (
	ErrSlugInUse    = errors.New("slug in use")
	ErrNotFound     = errors.New("clip not found")
	ErrBodyTooLarge = errors.New("body too large")
	ErrInvalidTTL   = errors.New("invalid ttl")
)

// Clip is a stored paste with an expiry anchored to creation time.
type Clip struct {
	Slug      string
	Body      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

const maxSecondsAsDuration = int64(math.MaxInt64 / int64(time.Second))

// TTLFromSeconds maps an API lifetime value to a preset duration.
func TTLFromSeconds(seconds int64) (time.Duration, error) {
	if seconds < 0 || seconds > maxSecondsAsDuration {
		return 0, ErrInvalidTTL
	}
	ttl := time.Duration(seconds) * time.Second
	for _, preset := range Presets {
		if ttl == preset {
			return ttl, nil
		}
	}
	return 0, ErrInvalidTTL
}

// ResolveTTL returns the chosen preset, defaulting when ttl is zero.
func ResolveTTL(ttl time.Duration) (time.Duration, error) {
	if ttl == 0 {
		return DefaultPreset, nil
	}
	for _, preset := range Presets {
		if ttl == preset {
			return ttl, nil
		}
	}
	return 0, ErrInvalidTTL
}
