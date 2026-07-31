package ratelimit

import (
	"testing"
	"time"
)

func TestMemoryLimiter_allowThenRefuse(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	l := NewMemoryLimiter(MemoryConfig{
		Rate:    3,
		Window:  time.Minute,
		MaxKeys: 100,
		Now:     func() time.Time { return now },
	})

	for i := 0; i < 3; i++ {
		d := l.Allow("client-a")
		if !d.Allowed {
			t.Fatalf("request %d: allowed = false, want true", i+1)
		}
	}

	d := l.Allow("client-a")
	if d.Allowed {
		t.Fatal("4th request: allowed = true, want false")
	}
	if d.RetryAfter <= 0 {
		t.Fatalf("RetryAfter = %v, want positive", d.RetryAfter)
	}
	if d.RetryAfter > time.Minute {
		t.Fatalf("RetryAfter = %v, want within window %v", d.RetryAfter, time.Minute)
	}
}

func TestMemoryLimiter_windowReset(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	current := now
	l := NewMemoryLimiter(MemoryConfig{
		Rate:    2,
		Window:  time.Minute,
		MaxKeys: 100,
		Now:     func() time.Time { return current },
	})

	l.Allow("client-a")
	l.Allow("client-a")
	if d := l.Allow("client-a"); d.Allowed {
		t.Fatal("3rd request in window: allowed = true, want false")
	}

	current = now.Add(time.Minute)
	if d := l.Allow("client-a"); !d.Allowed {
		t.Fatal("request after window: allowed = false, want true")
	}
}

func TestMemoryLimiter_evictionAtMaxKeys(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	l := NewMemoryLimiter(MemoryConfig{
		Rate:    10,
		Window:  time.Minute,
		MaxKeys: 3,
		Now:     func() time.Time { return now },
	})

	for _, key := range []string{"a", "b", "c"} {
		if d := l.Allow(key); !d.Allowed {
			t.Fatalf("key %q: allowed = false, want true", key)
		}
	}

	// Inserting a fourth key should evict the oldest (a).
	if d := l.Allow("d"); !d.Allowed {
		t.Fatal("key d: allowed = false, want true")
	}
	if len(l.keys) > 3 {
		t.Fatalf("len(keys) = %d, want <= 3", len(l.keys))
	}
}

func TestMemoryLimiter_evictionDoesNotOverLimit(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	l := NewMemoryLimiter(MemoryConfig{
		Rate:    2,
		Window:  time.Minute,
		MaxKeys: 2,
		Now:     func() time.Time { return now },
	})

	l.Allow("a")
	l.Allow("b")
	l.Allow("c") // evicts a

	if d := l.Allow("a"); !d.Allowed {
		t.Fatal("re-added a after eviction: allowed = false, want true")
	}
}
