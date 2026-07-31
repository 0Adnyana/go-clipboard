package httpapi

import (
	"net/http"
	"strconv"

	"github.com/0adnyana/go-clipboard/internal/ratelimit"
)

// RateLimit wraps a handler with an IP-keyed limiter check.
func RateLimit(limiter *ratelimit.MemoryLimiter, cfg ClientIPConfig, next http.HandlerFunc) http.HandlerFunc {
	if limiter == nil {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		key := ClientIP(r, cfg)
		decision := limiter.Allow(key)
		if !decision.Allowed {
			seconds := int(decision.RetryAfter.Seconds())
			if seconds < 1 {
				seconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		next(w, r)
	}
}
