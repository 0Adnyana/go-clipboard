package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0adnyana/go-clipboard/internal/clips"
	"github.com/0adnyana/go-clipboard/internal/clips/fake"
	"github.com/0adnyana/go-clipboard/internal/openapi"
	"github.com/0adnyana/go-clipboard/internal/ratelimit"
)

func newClipTestHandler(t *testing.T) http.Handler {
	return newClipTestHandlerWithRateLimit(t, RateLimitDependencies{})
}

func newClipTestHandlerWithRateLimit(t *testing.T, rl RateLimitDependencies) http.Handler {
	t.Helper()
	store := fake.NewStore()
	svc := clips.NewService(store)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := NewServer(logger, Dependencies{Clips: svc, RateLimit: rl})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv.Handler()
}

func newRateLimitTestHandler(t *testing.T, createRate, availRate int) http.Handler {
	t.Helper()
	store := fake.NewStore()
	svc := clips.NewService(store)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := ClientIPConfig{TrustedProxy: "127.0.0.1", IPv6Prefix: 64}
	rl := RateLimitDependencies{
		CreateLimiter: ratelimit.NewMemoryLimiter(ratelimit.MemoryConfig{
			Rate: createRate, Window: time.Minute, MaxKeys: 1000,
		}),
		AvailLimiter: ratelimit.NewMemoryLimiter(ratelimit.MemoryConfig{
			Rate: availRate, Window: time.Minute, MaxKeys: 1000,
		}),
		ClientIP: cfg,
	}
	srv, err := NewServer(logger, Dependencies{Clips: svc, RateLimit: rl})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv.Handler()
}

func withTrustedClient(req *http.Request, clientIP string) *http.Request {
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", clientIP)
	return req
}

func postClip(handler http.Handler, slug, body string, ttlSeconds ...int64) *httptest.ResponseRecorder {
	payload := map[string]any{"slug": slug, "body": body}
	if len(ttlSeconds) > 0 {
		payload["ttlSeconds"] = ttlSeconds[0]
	}
	raw, _ := json.Marshal(payload)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/clips", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	return rec
}

func getClip(handler http.Handler, slug string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/clips/"+slug, nil))
	return rec
}

func getAvailability(handler http.Handler, slug string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/clips/"+slug+"/availability", nil))
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
	var body openapi.CreateClipResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Slug != "my-clip" || body.ExpiresAt.IsZero() {
		t.Fatalf("body = %+v, want slug and expiresAt", body)
	}
}

func TestClips_createWithTTL(t *testing.T) {
	handler := newClipTestHandler(t)
	rec := postClip(handler, "short", "hello", 600)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var body openapi.CreateClipResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Slug != "short" {
		t.Fatalf("slug = %q, want short", body.Slug)
	}
}

func TestClips_createInvalidTTL(t *testing.T) {
	handler := newClipTestHandler(t)
	rec := postClip(handler, "bad-ttl", "hello", 900)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	body := decodeError(t, rec)
	if body.Code != "invalid_ttl" {
		t.Fatalf("code = %q, want invalid_ttl", body.Code)
	}
}

func TestClips_readSuccess(t *testing.T) {
	handler := newClipTestHandler(t)
	postClip(handler, "read-me", "payload")

	rec := getClip(handler, "read-me")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body openapi.ReadClipResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Slug != "read-me" || body.Body != "payload" {
		t.Fatalf("body = %+v", body)
	}
	if body.ServerTime.IsZero() {
		t.Fatal("serverTime is zero, want RFC3339Nano timestamp")
	}
}

func TestClips_availabilityFree(t *testing.T) {
	handler := newClipTestHandler(t)
	rec := getAvailability(handler, "free-name")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body openapi.ClipAvailabilityResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Slug != "free-name" || !body.Available {
		t.Fatalf("body = %+v, want available", body)
	}
}

func TestClips_availabilityLive(t *testing.T) {
	handler := newClipTestHandler(t)
	postClip(handler, "taken", "payload")

	rec := getAvailability(handler, "taken")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body openapi.ClipAvailabilityResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Available {
		t.Fatalf("body = %+v, want unavailable", body)
	}
}

func TestClips_availabilityReserved(t *testing.T) {
	handler := newClipTestHandler(t)
	rec := getAvailability(handler, "status")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	body := decodeError(t, rec)
	if body.Code != "reserved_slug" {
		t.Fatalf("code = %q, want reserved_slug", body.Code)
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

func TestClips_rateLimitCreate(t *testing.T) {
	handler := newRateLimitTestHandler(t, 2, 100)
	client := "203.0.113.50"

	for i := 0; i < 2; i++ {
		rec := postClipWithClient(handler, client, fmt.Sprintf("clip-%d", i), "body")
		if rec.Code != http.StatusCreated {
			t.Fatalf("request %d: status = %d, want 201", i+1, rec.Code)
		}
	}

	rec := postClipWithClient(handler, client, "clip-refused", "body")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header missing")
	}
	body := decodeError(t, rec)
	if body.Code != "rate_limited" {
		t.Fatalf("code = %q, want rate_limited", body.Code)
	}
	rec = getClipWithClient(handler, client, "clip-refused")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("refused clip read: status = %d, want 404", rec.Code)
	}
}

func TestClips_rateLimitAvailability(t *testing.T) {
	handler := newRateLimitTestHandler(t, 100, 2)
	client := "203.0.113.51"

	for i := 0; i < 2; i++ {
		rec := getAvailabilityWithClient(handler, client, fmt.Sprintf("probe-%d", i))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i+1, rec.Code)
		}
	}

	rec := getAvailabilityWithClient(handler, client, "probe-c")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header missing")
	}
	body := decodeError(t, rec)
	if body.Code != "rate_limited" {
		t.Fatalf("code = %q, want rate_limited", body.Code)
	}
}

func TestClips_rateLimitProxyClientIP(t *testing.T) {
	handler := newRateLimitTestHandler(t, 1, 100)

	rec := postClipWithClient(handler, "203.0.113.60", "first", "body")
	if rec.Code != http.StatusCreated {
		t.Fatalf("first request: status = %d, want 201", rec.Code)
	}

	// Same real client through trusted proxy should share the limit key.
	rec = postClipWithClient(handler, "203.0.113.60", "second", "body")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: status = %d, want 429", rec.Code)
	}

	// Different client IP should not be limited.
	rec = postClipWithClient(handler, "203.0.113.61", "other", "body")
	if rec.Code != http.StatusCreated {
		t.Fatalf("other client: status = %d, want 201", rec.Code)
	}
}

func TestClips_rateLimitDoesNotBreakClaimOrRead(t *testing.T) {
	handler := newRateLimitTestHandler(t, 10, 10)
	client := "203.0.113.70"

	rec := postClipWithClient(handler, client, "claim-me", "payload")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, want 201", rec.Code)
	}

	rec = getClipWithClient(handler, client, "claim-me")
	if rec.Code != http.StatusOK {
		t.Fatalf("read: status = %d, want 200", rec.Code)
	}

	postClipWithClient(handler, client, "claim-me", "other")
	rec = getClipWithClient(handler, client, "claim-me")
	if rec.Code != http.StatusOK {
		t.Fatalf("read after conflict attempt: status = %d, want 200", rec.Code)
	}

	rec = getClipWithClient(handler, client, "missing")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("not found: status = %d, want 404", rec.Code)
	}
}

func postClipWithClient(handler http.Handler, clientIP, slug, body string) *httptest.ResponseRecorder {
	payload := map[string]any{"slug": slug, "body": body}
	raw, _ := json.Marshal(payload)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/clips", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	withTrustedClient(req, clientIP)
	handler.ServeHTTP(rec, req)
	return rec
}

func getAvailabilityWithClient(handler http.Handler, clientIP, slug string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/clips/"+slug+"/availability", nil)
	withTrustedClient(req, clientIP)
	handler.ServeHTTP(rec, req)
	return rec
}

func getClipWithClient(handler http.Handler, clientIP, slug string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/clips/"+slug, nil)
	withTrustedClient(req, clientIP)
	handler.ServeHTTP(rec, req)
	return rec
}

func TestOpenAPIContract_readResponseShape(t *testing.T) {
	handler := newClipTestHandler(t)
	postClip(handler, "contract", "data")

	rec := getClip(handler, "contract")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body openapi.ReadClipResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode ReadClipResponse: %v", err)
	}
	if body.ExpiresAt.Before(body.ServerTime) {
		t.Fatalf("expiresAt %v before serverTime %v", body.ExpiresAt, body.ServerTime)
	}
	if body.ExpiresAt.Sub(body.ServerTime) > clips.DefaultPreset+time.Minute {
		t.Fatalf("expiry gap %v exceeds ceiling", body.ExpiresAt.Sub(body.ServerTime))
	}
}
