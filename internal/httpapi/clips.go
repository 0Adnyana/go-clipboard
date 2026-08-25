package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/0adnyana/go-clipboard/internal/clips"
	"github.com/0adnyana/go-clipboard/internal/openapi"
	"github.com/0adnyana/go-clipboard/internal/slug"
	"io"
	"log/slog"
	"net/http"
)

const maxCreateBodyBytes = clips.MaxBodyBytes + 4096 // room for JSON framing

// openapiServer implements the generated contract for clip and health handlers.
type openapiServer struct {
	logger *slog.Logger
	clips  *clips.Service
	health HealthDependencies
}

var _ openapi.ServerInterface = (*openapiServer)(nil)

func (s *openapiServer) CreateClip(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCreateBodyBytes)

	var req openapi.CreateClipRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusBadRequest, "body_too_large", "clip body must be at most 256 KiB")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON with slug and body fields")
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON with slug and body fields")
		return
	}

	in := clips.CreateInput{
		Slug: req.Slug,
		Body: req.Body,
	}
	if req.TtlSeconds != nil {
		ttl, err := clips.TTLFromSeconds(int64(*req.TtlSeconds))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_ttl", "lifetime must be 10 minutes, 1 hour, or 2 hours")
			return
		}
		in.TTL = ttl
	}

	result, err := s.clips.Create(r.Context(), in)
	if err != nil {
		writeClipError(w, s.logger, err)
		return
	}

	writeJSON(w, http.StatusCreated, openapi.CreateClipResponse{
		Slug:      result.Slug,
		ExpiresAt: result.ExpiresAt.UTC(),
	})
}

func (s *openapiServer) GetClip(w http.ResponseWriter, r *http.Request, slugStr string) {
	if slugStr == "" {
		writeError(w, http.StatusBadRequest, "invalid_slug", "slug is required")
		return
	}

	clip, err := s.clips.Read(r.Context(), slugStr)
	if err != nil {
		writeClipError(w, s.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, openapi.ReadClipResponse{
		Slug:       clip.Slug,
		Body:       clip.Body,
		ExpiresAt:  clip.ExpiresAt.UTC(),
		ServerTime: s.clips.Now(),
	})
}

func (s *openapiServer) GetClipAvailability(w http.ResponseWriter, r *http.Request, slugStr string) {
	if slugStr == "" {
		writeError(w, http.StatusBadRequest, "invalid_slug", "slug is required")
		return
	}

	available, err := s.clips.Availability(r.Context(), slugStr)
	if err != nil {
		writeClipError(w, s.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, openapi.ClipAvailabilityResponse{
		Slug:      slugStr,
		Available: available,
	})
}

func (s *openapiServer) GetHealth(w http.ResponseWriter, r *http.Request) {
	resp := buildHealthResponse(r.Context(), s.health)
	writeJSON(w, http.StatusOK, resp)
}

func writeClipError(w http.ResponseWriter, logger *slog.Logger, err error) {
	var slugErr *slug.ValidationError
	switch {
	case errors.As(err, &slugErr):
		writeError(w, http.StatusBadRequest, slugErr.Code, slugErr.Message)
	case errors.Is(err, clips.ErrBodyTooLarge):
		writeError(w, http.StatusBadRequest, "body_too_large", "clip body must be at most 256 KiB")
	case errors.Is(err, clips.ErrInvalidTTL):
		writeError(w, http.StatusBadRequest, "invalid_ttl", "lifetime must be 10 minutes, 1 hour, or 2 hours")
	case errors.Is(err, clips.ErrSlugInUse):
		writeError(w, http.StatusConflict, "slug_in_use", "that name is in use right now")
	case errors.Is(err, clips.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	default:
		writeInternalError(w, logger, err)
	}
}

func clipRoutes(logger *slog.Logger, svc *clips.Service, health HealthDependencies, rl RateLimitDependencies) []apiRoute {
	api := &openapiServer{
		logger: logger,
		clips:  svc,
		health: health,
	}
	return []apiRoute{
		{http.MethodPost, "/api/clips", RateLimit(rl.CreateLimiter, rl.ClientIP, api.CreateClip)},
		{http.MethodGet, "/api/clips/{slug}", func(w http.ResponseWriter, r *http.Request) {
			api.GetClip(w, r, r.PathValue("slug"))
		}},
		{http.MethodGet, "/api/clips/{slug}/availability", RateLimit(rl.AvailLimiter, rl.ClientIP, func(w http.ResponseWriter, r *http.Request) {
			api.GetClipAvailability(w, r, r.PathValue("slug"))
		})},
	}
}
