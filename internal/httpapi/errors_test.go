package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteErrorJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusNotFound, "not_found", "resource not found")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "not_found" || body.Message != "resource not found" {
		t.Fatalf("body = %+v, want not_found / resource not found", body)
	}
}

func TestWriteInternalError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rec := httptest.NewRecorder()
	writeInternalError(rec, logger, io.ErrUnexpectedEOF)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	var body ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "internal_error" {
		t.Fatalf("code = %q, want internal_error", body.Code)
	}
	if strings.Contains(body.Message, "EOF") {
		t.Fatalf("message leaked underlying error: %q", body.Message)
	}
}

func TestRequestLogging_emitsOneRecord(t *testing.T) {
	var logs strings.Builder
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	handler := RequestLogging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	output := logs.String()
	if !strings.Contains(output, "request completed") {
		t.Fatalf("logs = %q, want request completed record", output)
	}
	if strings.Count(output, "request completed") != 1 {
		t.Fatalf("expected exactly one completion log, got: %q", output)
	}
}

func TestPanicRecovery_returns500AndContinuesServing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	panicHandler := PanicRecovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	okHandler := PanicRecovery(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	panicHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/panic", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("panic response status = %d, want 500", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	okHandler.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/ok", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("follow-up response status = %d, want 200", rec2.Code)
	}
}
