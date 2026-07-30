package sweeper

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/0adnyana/go-clipboard/internal/clips"
	"github.com/0adnyana/go-clipboard/internal/clips/fake"
)

type failingDeleter struct {
	store   *fake.Store
	fail    bool
	mu      sync.Mutex
	attempt int
}

func (f *failingDeleter) DeleteExpired(ctx context.Context) (int64, error) {
	f.mu.Lock()
	f.attempt++
	f.mu.Unlock()
	if f.fail {
		return 0, errors.New("database unavailable")
	}
	return f.store.DeleteExpiredClips(ctx)
}

func TestSweeper_deletesOnlyExpiredRows(t *testing.T) {
	store := fake.NewStore()
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	store.SetNow(func() time.Time { return now })

	svc := clips.NewService(store)
	svc.SetNow(func() time.Time { return now })
	ctx := context.Background()

	_, err := svc.Create(ctx, clips.CreateInput{Slug: "live", Body: "keep"})
	if err != nil {
		t.Fatalf("Create(live) = %v", err)
	}

	_, err = svc.Create(ctx, clips.CreateInput{Slug: "dead", Body: "gone", TTL: clips.Preset10m})
	if err != nil {
		t.Fatalf("Create(dead) = %v", err)
	}

	now = now.Add(clips.Preset10m + time.Minute)
	store.SetNow(func() time.Time { return now })
	svc.SetNow(func() time.Time { return now })

	deleted, err := svc.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired() = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	if _, err := svc.Read(ctx, "live"); err != nil {
		t.Fatalf("Read(live) = %v, want success", err)
	}
	if _, err := svc.Read(ctx, "dead"); !errors.Is(err, clips.ErrNotFound) {
		t.Fatalf("Read(dead) = %v, want ErrNotFound", err)
	}
}

func TestSweeper_failedPassDoesNotStopLoop(t *testing.T) {
	store := fake.NewStore()
	deleter := &failingDeleter{store: store, fail: true}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := New(deleter, 10*time.Millisecond, logger)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Run(ctx)
	}()

	deadline := time.Now().Add(50 * time.Millisecond)
	for attempts := 0; attempts < 2 && time.Now().Before(deadline); {
		deleter.mu.Lock()
		attempts = deleter.attempt
		deleter.mu.Unlock()
		if attempts >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	wg.Wait()

	deleter.mu.Lock()
	attempts := deleter.attempt
	deleter.mu.Unlock()
	if attempts < 2 {
		t.Fatalf("attempts = %d, want at least 2", attempts)
	}
}

func TestSweeper_correctnessIndependentOfRunning(t *testing.T) {
	store := fake.NewStore()
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	store.SetNow(func() time.Time { return now })

	svc := clips.NewService(store)
	svc.SetNow(func() time.Time { return now })
	ctx := context.Background()

	_, err := svc.Create(ctx, clips.CreateInput{Slug: "reuse", Body: "old"})
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}

	now = now.Add(clips.DefaultPreset + time.Minute)
	store.SetNow(func() time.Time { return now })
	svc.SetNow(func() time.Time { return now })

	if _, err := svc.Read(ctx, "reuse"); !errors.Is(err, clips.ErrNotFound) {
		t.Fatalf("Read(expired) = %v, want ErrNotFound", err)
	}

	result, err := svc.Create(ctx, clips.CreateInput{Slug: "reuse", Body: "new"})
	if err != nil {
		t.Fatalf("reclaim Create() = %v", err)
	}

	clip, err := svc.Read(ctx, result.Slug)
	if err != nil {
		t.Fatalf("Read(reclaimed) = %v", err)
	}
	if clip.Body != "new" {
		t.Fatalf("body = %q, want new", clip.Body)
	}
}
