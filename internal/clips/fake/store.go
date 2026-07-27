package fake

import (
	"context"
	"sync"
	"time"

	"github.com/0adnyana/go-clipboard/internal/clips"
)

// Store is an in-memory clip store for unit tests.
type Store struct {
	mu    sync.Mutex
	clips map[string]storedClip
	now   func() time.Time
}

type storedClip struct {
	slug      string
	body      string
	createdAt time.Time
	expiresAt time.Time
}

func NewStore() *Store {
	return &Store{
		clips: make(map[string]storedClip),
		now:   time.Now,
	}
}

func (s *Store) SetNow(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = fn
}

func (s *Store) ClaimClip(ctx context.Context, slug, body string, createdAt, expiresAt time.Time) (*clips.Clip, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.clips[slug]
	if !ok {
		s.clips[slug] = storedClip{
			slug:      slug,
			body:      body,
			createdAt: createdAt,
			expiresAt: expiresAt,
		}
		return &clips.Clip{
			Slug:      slug,
			Body:      body,
			CreatedAt: createdAt,
			ExpiresAt: expiresAt,
		}, nil
	}

	if existing.expiresAt.After(s.now()) {
		return nil, clips.ErrSlugInUse
	}

	s.clips[slug] = storedClip{
		slug:      slug,
		body:      body,
		createdAt: createdAt,
		expiresAt: expiresAt,
	}
	return &clips.Clip{
		Slug:      slug,
		Body:      body,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *Store) GetLiveClip(ctx context.Context, slug string) (*clips.Clip, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.clips[slug]
	if !ok || !existing.expiresAt.After(s.now()) {
		return nil, clips.ErrNotFound
	}
	return &clips.Clip{
		Slug:      existing.slug,
		Body:      existing.body,
		CreatedAt: existing.createdAt,
		ExpiresAt: existing.expiresAt,
	}, nil
}
