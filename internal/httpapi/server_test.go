package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestHandler() http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(logger, Dependencies{}).Handler()
}

func TestRouting_registeredRouteIsServed(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
}

func TestRouting_fallbacksAnswerInJSON(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantAllow  string
	}{
		{"unknown api path", http.MethodGet, "/api/does-not-exist", http.StatusNotFound, ""},
		// The method must not decide this one: an unregistered path is missing
		// regardless of what the caller tried to do to it.
		{"unknown api path with a write method", http.MethodPost, "/api/does-not-exist", http.StatusNotFound, ""},
		{"wrong method on a registered path", http.MethodPost, "/api/health", http.StatusMethodNotAllowed, "GET, HEAD"},
		{"root", http.MethodGet, "/", http.StatusNotFound, ""},
		{"user-owned slug", http.MethodGet, "/some-user-key", http.StatusNotFound, ""},
		{"user-owned slug with a write method", http.MethodDelete, "/some-user-key", http.StatusNotFound, ""},
	}

	handler := newTestHandler()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", ct)
			}
			if allow := rec.Header().Get("Allow"); allow != tc.wantAllow {
				t.Fatalf("Allow = %q, want %q", allow, tc.wantAllow)
			}

			var body ErrorResponse
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Code == "" || body.Message == "" {
				t.Fatalf("body = %+v, want an error code and message", body)
			}
		})
	}
}

func TestRouting_everyPathOutsideAPIGetsTheSameResponse(t *testing.T) {
	handler := newTestHandler()

	var first string
	for _, path := range []string{"/", "/some-user-key", "/assets/app.js", "/index.html"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", path, rec.Code)
		}
		if first == "" {
			first = rec.Body.String()
			continue
		}
		if rec.Body.String() != first {
			t.Fatalf("%s body = %q, want the same refusal as every other path (%q)", path, rec.Body.String(), first)
		}
	}
}
