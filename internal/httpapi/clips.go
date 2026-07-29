package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/0adnyana/go-clipboard/internal/clips"
	"github.com/0adnyana/go-clipboard/internal/slug"
)

const maxCreateBodyBytes = clips.MaxBodyBytes + 4096 // room for JSON framing

type createClipRequest struct {
	Slug string `json:"slug"`
	Body string `json:"body"`
}

type createClipResponse struct {
	Slug      string `json:"slug"`
	ExpiresAt string `json:"expiresAt"`
}

type readClipResponse struct {
	Slug      string `json:"slug"`
	Body      string `json:"body"`
	ExpiresAt string `json:"expiresAt"`
}

func handleCreateClip(logger *slog.Logger, svc *clips.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxCreateBodyBytes)

		var req createClipRequest
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

		result, err := svc.Create(r.Context(), clips.CreateInput{
			Slug: req.Slug,
			Body: req.Body,
		})
		if err != nil {
			writeClipError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusCreated, createClipResponse{
			Slug:      result.Slug,
			ExpiresAt: result.ExpiresAt.UTC().Format(time.RFC3339Nano),
		})
	}
}

func handleReadClip(logger *slog.Logger, svc *clips.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slugStr := r.PathValue("slug")
		if slugStr == "" {
			writeError(w, http.StatusBadRequest, "invalid_slug", "slug is required")
			return
		}

		clip, err := svc.Read(r.Context(), slugStr)
		if err != nil {
			writeClipError(w, logger, err)
			return
		}

		writeJSON(w, http.StatusOK, readClipResponse{
			Slug:      clip.Slug,
			Body:      clip.Body,
			ExpiresAt: clip.ExpiresAt.UTC().Format(time.RFC3339Nano),
		})
	}
}

func writeClipError(w http.ResponseWriter, logger *slog.Logger, err error) {
	var slugErr *slug.ValidationError
	switch {
	case errors.As(err, &slugErr):
		writeError(w, http.StatusBadRequest, slugErr.Code, slugErr.Message)
	case errors.Is(err, clips.ErrBodyTooLarge):
		writeError(w, http.StatusBadRequest, "body_too_large", "clip body must be at most 256 KiB")
	case errors.Is(err, clips.ErrSlugInUse):
		writeError(w, http.StatusConflict, "slug_in_use", "that name is in use right now")
	case errors.Is(err, clips.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	default:
		writeInternalError(w, logger, err)
	}
}

func clipRoutes(logger *slog.Logger, svc *clips.Service) []apiRoute {
	return []apiRoute{
		{http.MethodPost, "/api/clips", handleCreateClip(logger, svc)},
		{http.MethodGet, "/api/clips/{slug}", handleReadClip(logger, svc)},
	}
}
