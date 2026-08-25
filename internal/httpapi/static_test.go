package httpapi

import (
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/0adnyana/go-clipboard/internal/clips"
	"github.com/0adnyana/go-clipboard/internal/clips/fake"
)

func testStaticFS() fs.FS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>app</title>")},
		"assets/app.js": &fstest.MapFile{
			Data: []byte("console.log('ok')"),
		},
	}
}

func newStaticTestHandler(t *testing.T) http.Handler {
	t.Helper()
	srv, err := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Dependencies{
			Clips:    clips.NewService(fake.NewStore()),
			StaticFS: testStaticFS(),
		},
	)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv.Handler()
}

func TestStatic_servesEmbeddedAsset(t *testing.T) {
	rec := httptest.NewRecorder()
	newStaticTestHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "console.log('ok')" {
		t.Fatalf("body = %q, want embedded asset", body)
	}
}

func TestStatic_spaFallbackForUnknownPath(t *testing.T) {
	rec := httptest.NewRecorder()
	newStaticTestHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/some-user-key", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "<!doctype html><title>app</title>" {
		t.Fatalf("body = %q, want index.html fallback", body)
	}
}

func TestStatic_apiHealthUnaffected(t *testing.T) {
	rec := httptest.NewRecorder()
	newStaticTestHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusServiceUnavailable {
		// No health deps wired → degraded → 503; still JSON from the health handler.
		t.Fatalf("status = %d, want 503 from health handler", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
}

func TestStatic_apiWinsRegardlessOfRegistrationOrder(t *testing.T) {
	// /api/does-not-exist must be the API JSON 404, not SPA index.html.
	rec := httptest.NewRecorder()
	newStaticTestHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json (API fallback, not SPA)", ct)
	}
}

func TestRequireHost_rejectsMismatch(t *testing.T) {
	srv, err := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Dependencies{
			Clips:       clips.NewService(fake.NewStore()),
			StaticFS:    testStaticFS(),
			TLSHostname: "clip.example.com",
		},
	)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Host = "evil.example.com"
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusMisdirectedRequest {
		t.Fatalf("status = %d, want 421", rec.Code)
	}
}

func TestRequireHost_allowsConfiguredHost(t *testing.T) {
	srv, err := NewServer(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Dependencies{
			Clips:       clips.NewService(fake.NewStore()),
			StaticFS:    testStaticFS(),
			TLSHostname: "clip.example.com",
		},
	)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "clip.example.com"
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
