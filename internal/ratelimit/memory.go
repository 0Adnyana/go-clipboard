package ratelimit

import (
	"sync"
	"time"
)

type entry struct {
	count       int
	windowStart time.Time
}

// MemoryConfig configures a fixed-window in-memory limiter.
type MemoryConfig struct {
	Rate    int
	Window  time.Duration
	// MaxKeys is the hard cap on distinct tracked keys. Values less than or
	// equal to zero disable key bounding and eviction; makeRoom becomes a no-op.
	MaxKeys int
	Now     func() time.Time
}

// NewMemoryLimiter returns a concurrency-safe fixed-window limiter with bounded keys.
func NewMemoryLimiter(cfg MemoryConfig) Limiter {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &memoryLimiter{
		rate:    cfg.Rate,
		window:  cfg.Window,
		maxKeys: cfg.MaxKeys,
		now:     now,
		keys:    make(map[string]entry),
	}
}

type memoryLimiter struct {
	mu      sync.Mutex
	rate    int
	window  time.Duration
	maxKeys int
	now     func() time.Time
	keys    map[string]entry
}

func (l *memoryLimiter) Allow(key string) Decision {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()

	e, ok := l.keys[key]
	if !ok || now.Sub(e.windowStart) >= l.window {
		if !ok {
			l.makeRoom(now)
		}
		l.keys[key] = entry{count: 1, windowStart: now}
		return Decision{Allowed: true}
	}

	if e.count >= l.rate {
		remaining := l.window - now.Sub(e.windowStart)
		if remaining < time.Second {
			remaining = time.Second
		}
		return Decision{Allowed: false, RetryAfter: remaining}
	}

	e.count++
	l.keys[key] = e
	return Decision{Allowed: true}
}

func (l *memoryLimiter) makeRoom(now time.Time) {
	if l.maxKeys <= 0 || len(l.keys) < l.maxKeys {
		return
	}
	var oldestKey string
	var oldestStart time.Time
	first := true
	for k, e := range l.keys {
		if first || e.windowStart.Before(oldestStart) {
			oldestKey = k
			oldestStart = e.windowStart
			first = false
		}
	}
	if oldestKey != "" {
		delete(l.keys, oldestKey)
	}
}
