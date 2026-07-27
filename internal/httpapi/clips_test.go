package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0adnyana/go-clipboard/internal/clips"
	"github.com/0adnyana/go-clipboard/internal/clips/fake"
)

func newClipTestHandler(t *testing.T) http.Handler {
	t.Helper()
	store := fake.NewStore()
	svc := clips.NewService(store)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(logger, Dependencies{ClipService: svc}).Handler()
}

func postClip(handler http.Handler, slug, body string) *httptest.ResponseRecorder {
	payload, _ := json.Marshal(map[string]string{"slug": slug, "body": body})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/clips", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	return rec
}

func getClip(handler http.Handler, slug string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/clips/"+slug, nil))
	return rec
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var body ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return body
}

func TestClips_createSuccess(t *testing.T) {
	handler := newClipTestHandler(t)
	rec := postClip(handler, "my-clip", "hello")

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var body createClipResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Slug != "my-clip" || body.ExpiresAt == "" {
		t.Fatalf("body = %+v, want slug and expiresAt", body)
	}
}

func TestClips_readSuccess(t *testing.T) {
	handler := newClipTestHandler(t)
	postClip(handler, "read-me", "payload")

	rec := getClip(handler, "read-me")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body readClipResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Slug != "read-me" || body.Body != "payload" {
		t.Fatalf("body = %+v", body)
	}
}

func TestClips_validationErrors(t *testing.T) {
	handler := newClipTestHandler(t)
	cases := []struct {
		name       string
		slug       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"too short slug", "ab", "x", http.StatusBadRequest, "invalid_slug"},
		{"illegal slug", "bad slug", "x", http.StatusBadRequest, "invalid_slug"},
		{"reserved slug", "status", "x", http.StatusBadRequest, "reserved_slug"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postClip(handler, tc.slug, tc.body)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			body := decodeError(t, rec)
			if body.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", body.Code, tc.wantCode)
			}
		})
	}
}

func TestClips_conflict(t *testing.T) {
	handler := newClipTestHandler(t)
	postClip(handler, "taken", "first")
	rec := postClip(handler, "taken", "second")

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	body := decodeError(t, rec)
	if body.Code != "slug_in_use" || body.Message != "that name is in use right now" {
		t.Fatalf("body = %+v", body)
	}
}

func TestClips_notFound(t *testing.T) {
	handler := newClipTestHandler(t)
	rec := getClip(handler, "missing")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body := decodeError(t, rec)
	if body.Code != "not_found" {
		t.Fatalf("code = %q, want not_found", body.Code)
	}
}

func TestClips_bodyTooLarge(t *testing.T) {
	handler := newClipTestHandler(t)
	rec := postClip(handler, "big", strings.Repeat("x", clips.MaxBodyBytes+1))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	body := decodeError(t, rec)
	if body.Code != "body_too_large" {
		t.Fatalf("code = %q, want body_too_large", body.Code)
	}
}

func TestClips_invalidJSON(t *testing.T) {
	handler := newClipTestHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/clips", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	body := decodeError(t, rec)
	if body.Code != "invalid_body" {
		t.Fatalf("code = %q, want invalid_body", body.Code)
	}
}
