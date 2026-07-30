package clips

import (
	"context"
	"errors"
	"time"

	"github.com/0adnyana/go-clipboard/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// PGStore adapts generated sqlc queries to the clips Store interface.
type PGStore struct {
	queries db.Querier
}

func NewPGStore(queries db.Querier) *PGStore {
	return &PGStore{queries: queries}
}

func (s *PGStore) ClaimClip(ctx context.Context, slug, body string, createdAt, expiresAt time.Time) (*Clip, error) {
	row, err := s.queries.ClaimClip(ctx, db.ClaimClipParams{
		Slug: slug,
		Body: body,
		CreatedAt: pgtype.Timestamptz{
			Time:  createdAt,
			Valid: true,
		},
		ExpiresAt: pgtype.Timestamptz{
			Time:  expiresAt,
			Valid: true,
		},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSlugInUse
		}
		return nil, err
	}
	return clipFromRow(row), nil
}

func (s *PGStore) GetLiveClip(ctx context.Context, slug string) (*Clip, error) {
	row, err := s.queries.GetLiveClip(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return clipFromRow(row), nil
}

func (s *PGStore) DeleteExpiredClips(ctx context.Context) (int64, error) {
	return s.queries.DeleteExpiredClips(ctx)
}

func (s *PGStore) SlugIsLive(ctx context.Context, slug string) (bool, error) {
	return s.queries.SlugIsLive(ctx, slug)
}

func clipFromRow(row db.Clip) *Clip {
	return &Clip{
		Slug:      row.Slug,
		Body:      row.Body,
		CreatedAt: row.CreatedAt.Time,
		ExpiresAt: row.ExpiresAt.Time,
	}
}
