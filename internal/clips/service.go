package clips

import (
	"context"
	"errors"
	"time"

	"github.com/0adnyana/go-clipboard/internal/slug"
)

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{
		store: store,
		now:   time.Now,
	}
}

func (s *Service) SetNow(fn func() time.Time) {
	s.now = fn
}

type CreateInput struct {
	Slug string
	Body string
}

type CreateResult struct {
	Slug      string
	ExpiresAt time.Time
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*CreateResult, error) {
	if err := slug.Validate(in.Slug); err != nil {
		return nil, err
	}
	if len(in.Body) > MaxBodyBytes {
		return nil, ErrBodyTooLarge
	}

	createdAt := s.now().UTC()
	expiresAt := createdAt.Add(Lease)

	clip, err := s.store.ClaimClip(ctx, in.Slug, in.Body, createdAt, expiresAt)
	if err != nil {
		if errors.Is(err, ErrSlugInUse) {
			return nil, ErrSlugInUse
		}
		return nil, err
	}

	return &CreateResult{
		Slug:      clip.Slug,
		ExpiresAt: clip.ExpiresAt,
	}, nil
}

func (s *Service) Read(ctx context.Context, slugStr string) (*Clip, error) {
	clip, err := s.store.GetLiveClip(ctx, slugStr)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return clip, nil
}
