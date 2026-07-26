package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{
		Code:    code,
		Message: message,
	})
}

func writeInternalError(w http.ResponseWriter, logger *slog.Logger, err error) {
	logger.Error("internal server error", slog.Any("error", err))
	writeError(w, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
}
