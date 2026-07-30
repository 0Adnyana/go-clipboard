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

	now = now.Add(clips.DefaultPreset + time.Minute)
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

	expired := fixedNow().Add(clips.DefaultPreset + time.Second)
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
	want := now.Add(clips.DefaultPreset)
	if !result.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", result.ExpiresAt, want)
	}
}

func TestCreate_eachPresetAnchorsExpiry(t *testing.T) {
	cases := []struct {
		name string
		ttl  time.Duration
	}{
		{"10m", clips.Preset10m},
		{"1h", clips.Preset1h},
		{"2h", clips.Preset2h},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newTestService()
			now := fixedNow()

			result, err := svc.Create(context.Background(), clips.CreateInput{
				Slug: "preset-" + tc.name,
				Body: "x",
				TTL:  tc.ttl,
			})
			if err != nil {
				t.Fatalf("Create() = %v", err)
			}
			want := now.Add(tc.ttl)
			if !result.ExpiresAt.Equal(want) {
				t.Fatalf("ExpiresAt = %v, want %v", result.ExpiresAt, want)
			}
		})
	}
}

func TestCreate_rejectsInvalidTTL(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.Create(context.Background(), clips.CreateInput{
		Slug: "bad-ttl",
		Body: "x",
		TTL:  30 * time.Minute,
	})
	if !errors.Is(err, clips.ErrInvalidTTL) {
		t.Fatalf("Create() = %v, want ErrInvalidTTL", err)
	}
}

func TestCreate_liveReclaimDoesNotResetExpiry(t *testing.T) {
	store := fake.NewStore()
	now := fixedNow()
	store.SetNow(func() time.Time { return now })
	svc := clips.NewService(store)
	svc.SetNow(func() time.Time { return now })

	ctx := context.Background()
	first, err := svc.Create(ctx, clips.CreateInput{Slug: "reuse", Body: "old", TTL: clips.Preset10m})
	if err != nil {
		t.Fatalf("first Create() = %v", err)
	}

	now = now.Add(clips.Preset10m + time.Minute)
	store.SetNow(func() time.Time { return now })
	svc.SetNow(func() time.Time { return now })

	second, err := svc.Create(ctx, clips.CreateInput{Slug: "reuse", Body: "new", TTL: clips.Preset1h})
	if err != nil {
		t.Fatalf("reclaim Create() = %v", err)
	}
	if second.ExpiresAt.Equal(first.ExpiresAt) {
		t.Fatalf("reclaim reused old expiry %v", second.ExpiresAt)
	}
	want := now.Add(clips.Preset1h)
	if !second.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", second.ExpiresAt, want)
	}
}

func TestAvailability_freeName(t *testing.T) {
	svc, _ := newTestService()
	available, err := svc.Availability(context.Background(), "free-name")
	if err != nil {
		t.Fatalf("Availability() = %v", err)
	}
	if !available {
		t.Fatal("available = false, want true")
	}
}

func TestAvailability_liveName(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	_, err := svc.Create(ctx, clips.CreateInput{Slug: "taken", Body: "x"})
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}

	available, err := svc.Availability(ctx, "taken")
	if err != nil {
		t.Fatalf("Availability() = %v", err)
	}
	if available {
		t.Fatal("available = true, want false")
	}
}

func TestAvailability_invalidSlug(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.Availability(context.Background(), "ab")
	var ve *slug.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Availability() = %v, want ValidationError", err)
	}
}

func TestAvailability_reservedSlug(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.Availability(context.Background(), slug.Reserved[0])
	var ve *slug.ValidationError
	if !errors.As(err, &ve) || ve.Code != "reserved_slug" {
		t.Fatalf("Availability() = %v, want reserved_slug", err)
	}
}

func TestAvailability_doesNotMutateState(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	available, err := svc.Availability(ctx, "race-name")
	if err != nil || !available {
		t.Fatalf("Availability() = %v, %v", available, err)
	}

	const workers = 4
	var wg sync.WaitGroup
	wg.Add(workers)
	results := make(chan error, workers)
	for range workers {
		go func() {
			defer wg.Done()
			_, err := svc.Create(ctx, clips.CreateInput{Slug: "race-name", Body: "payload"})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	var wins int
	for err := range results {
		if err == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("wins = %d, want 1", wins)
	}
}
