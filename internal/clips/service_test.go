package clips_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0adnyana/go-clipboard/internal/clips"
	"github.com/0adnyana/go-clipboard/internal/clips/fake"
	"github.com/0adnyana/go-clipboard/internal/slug"
)

func fixedNow() time.Time {
	return time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
}

func newTestService() (*clips.Service, *fake.Store) {
	store := fake.NewStore()
	store.SetNow(fixedNow)
	svc := clips.NewService(store)
	svc.SetNow(fixedNow)
	return svc, store
}

func TestCreate_roundTripsWhitespaceExactly(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	body := "  leading\n\ttrailing  \n"

	result, err := svc.Create(ctx, clips.CreateInput{Slug: "my-clip", Body: body})
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}

	clip, err := svc.Read(ctx, result.Slug)
	if err != nil {
		t.Fatalf("Read() = %v", err)
	}
	if clip.Body != body {
		t.Fatalf("body = %q, want %q", clip.Body, body)
	}
}

func TestCreate_acceptsMaxSizeBody(t *testing.T) {
	svc, _ := newTestService()
	body := strings.Repeat("x", clips.MaxBodyBytes)

	_, err := svc.Create(context.Background(), clips.CreateInput{Slug: "max-size", Body: body})
	if err != nil {
		t.Fatalf("Create(max) = %v", err)
	}
}

func TestCreate_rejectsOversizeBody(t *testing.T) {
	svc, _ := newTestService()
	body := strings.Repeat("x", clips.MaxBodyBytes+1)

	_, err := svc.Create(context.Background(), clips.CreateInput{Slug: "too-big", Body: body})
	if !errors.Is(err, clips.ErrBodyTooLarge) {
		t.Fatalf("Create(oversize) = %v, want ErrBodyTooLarge", err)
	}
}

func TestCreate_liveCollisionMessage(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	_, err := svc.Create(ctx, clips.CreateInput{Slug: "taken", Body: "first"})
	if err != nil {
		t.Fatalf("first Create() = %v", err)
	}

	_, err = svc.Create(ctx, clips.CreateInput{Slug: "taken", Body: "second"})
	if !errors.Is(err, clips.ErrSlugInUse) {
		t.Fatalf("second Create() = %v, want ErrSlugInUse", err)
	}
}

func TestCreate_reclaimsExpiredSlug(t *testing.T) {
	store := fake.NewStore()
	now := fixedNow()
	store.SetNow(func() time.Time { return now })
	svc := clips.NewService(store)
	svc.SetNow(func() time.Time { return now })

	ctx := context.Background()
	_, err := svc.Create(ctx, clips.CreateInput{Slug: "reuse", Body: "old"})
	if err != nil {
		t.Fatalf("first Create() = %v", err)
	}

	now = now.Add(clips.Lease + time.Minute)
	store.SetNow(func() time.Time { return now })
	svc.SetNow(func() time.Time { return now })

	result, err := svc.Create(ctx, clips.CreateInput{Slug: "reuse", Body: "new"})
	if err != nil {
		t.Fatalf("reclaim Create() = %v", err)
	}

	clip, err := svc.Read(ctx, result.Slug)
	if err != nil {
		t.Fatalf("Read() = %v", err)
	}
	if clip.Body != "new" {
		t.Fatalf("body = %q, want %q", clip.Body, "new")
	}
}

func TestCreate_concurrentClaimsSingleWinner(t *testing.T) {
	store := fake.NewStore()
	store.SetNow(fixedNow)
	svc := clips.NewService(store)
	svc.SetNow(fixedNow)

	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)

	results := make(chan error, workers)
	for range workers {
		go func() {
			defer wg.Done()
			_, err := svc.Create(context.Background(), clips.CreateInput{
				Slug: "race",
				Body: "payload",
			})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	var wins, losses int
	for err := range results {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, clips.ErrSlugInUse):
			losses++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("wins = %d, want 1", wins)
	}
	if losses != workers-1 {
		t.Fatalf("losses = %d, want %d", losses, workers-1)
	}
}

func TestCreate_caseVariantsAreDistinct(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	_, err := svc.Create(ctx, clips.CreateInput{Slug: "notes", Body: "lower"})
	if err != nil {
		t.Fatalf("Create(notes) = %v", err)
	}
	_, err = svc.Create(ctx, clips.CreateInput{Slug: "Notes", Body: "mixed"})
	if err != nil {
		t.Fatalf("Create(Notes) = %v", err)
	}

	lower, err := svc.Read(ctx, "notes")
	if err != nil || lower.Body != "lower" {
		t.Fatalf("Read(notes) = %v, body %q", err, lower.Body)
	}
	mixed, err := svc.Read(ctx, "Notes")
	if err != nil || mixed.Body != "mixed" {
		t.Fatalf("Read(Notes) = %v, body %q", err, mixed.Body)
	}
}

func TestCreate_rejectsReservedSlug(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.Create(context.Background(), clips.CreateInput{
		Slug: slug.Reserved[0],
		Body: "hello",
	})
	var ve *slug.ValidationError
	if !errors.As(err, &ve) || ve.Code != "reserved_slug" {
		t.Fatalf("Create(reserved) = %v, want reserved_slug", err)
	}
}

func TestRead_exactSlugMatchOnly(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	_, err := svc.Create(ctx, clips.CreateInput{Slug: "exact", Body: "data"})
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}

	_, err = svc.Read(ctx, "Exact")
	if !errors.Is(err, clips.ErrNotFound) {
		t.Fatalf("Read(Exact) = %v, want ErrNotFound", err)
	}
}

func TestRead_missingAndExpiredSameError(t *testing.T) {
	svc, store := newTestService()
	ctx := context.Background()

	_, err := svc.Read(ctx, "never")
	if !errors.Is(err, clips.ErrNotFound) {
		t.Fatalf("Read(missing) = %v, want ErrNotFound", err)
	}

	_, err = svc.Create(ctx, clips.CreateInput{Slug: "gone", Body: "temp"})
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}

	expired := fixedNow().Add(clips.Lease + time.Second)
	store.SetNow(func() time.Time { return expired })
	svc.SetNow(func() time.Time { return expired })

	_, err = svc.Read(ctx, "gone")
	if !errors.Is(err, clips.ErrNotFound) {
		t.Fatalf("Read(expired) = %v, want ErrNotFound", err)
	}
}

func TestRead_returnsBodyUnchanged(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	body := "\tkeep\tthis\n"

	_, err := svc.Create(ctx, clips.CreateInput{Slug: "raw-body", Body: body})
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}

	clip, err := svc.Read(ctx, "raw-body")
	if err != nil {
		t.Fatalf("Read() = %v", err)
	}
	if clip.Body != body {
		t.Fatalf("body = %q, want %q", clip.Body, body)
	}
}

func TestCreate_setsTwoHourExpiry(t *testing.T) {
	svc, _ := newTestService()
	now := fixedNow()

	result, err := svc.Create(context.Background(), clips.CreateInput{Slug: "ttl", Body: "x"})
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}
	want := now.Add(clips.Lease)
	if !result.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", result.ExpiresAt, want)
	}
}
