package ratelimit

import "time"

// Decision is whether a key may act now and, when refused, how long to wait.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Limiter answers whether a key may act and how long to wait on refusal.
type Limiter interface {
	Allow(key string) Decision
}
