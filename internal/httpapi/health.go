package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/0adnyana/go-clipboard/internal/clips"
	"github.com/0adnyana/go-clipboard/internal/database"
	"github.com/0adnyana/go-clipboard/internal/db"
	appmigrate "github.com/0adnyana/go-clipboard/internal/migrate"
)

// MigrationChecker reports migration state without applying anything. The
// handler takes it as a dependency rather than building one per request, so the
// goose provider and its database/sql handle are created once during wiring.
type MigrationChecker interface {
	CheckPending(ctx context.Context) (appmigrate.Status, error)
}

type Dependencies struct {
	Pool        *database.Pool
	Migrations  MigrationChecker
	Queries     db.Querier
	ClipService *clips.Service
}

type DatabaseHealth struct {
	Reachable bool   `json:"reachable"`
	LatencyMs *int64 `json:"latencyMs,omitempty"`
}

type MigrationHealth struct {
	Pending        bool  `json:"pending"`
	CurrentVersion int64 `json:"currentVersion"`
}

type HealthResponse struct {
	Status   string         `json:"status"`
	Database DatabaseHealth `json:"database"`
	// Absent when the pending check could not run. A zero-valued block would
	// claim the schema is current at version 0, which is the reading a developer
	// is most likely to act on and the one we have no evidence for.
	Migrations *MigrationHealth `json:"migrations,omitempty"`
	ServerTime *string          `json:"serverTime,omitempty"`
}

func buildHealthResponse(ctx context.Context, deps Dependencies) HealthResponse {
	resp := HealthResponse{
		Status: "ok",
		Database: DatabaseHealth{
			Reachable: false,
		},
	}

	if deps.Pool != nil {
		probe := deps.Pool.Probe(ctx)
		resp.Database.Reachable = probe.Reachable
		if probe.Reachable {
			latencyMs := probe.Latency.Milliseconds()
			resp.Database.LatencyMs = &latencyMs
		} else {
			resp.Status = "degraded"
		}
	} else {
		resp.Status = "degraded"
	}

	if deps.Migrations != nil {
		status, err := deps.Migrations.CheckPending(ctx)
		if err == nil {
			resp.Migrations = &MigrationHealth{
				Pending:        status.Pending,
				CurrentVersion: status.CurrentVersion,
			}
			if status.Pending {
				resp.Status = "degraded"
			}
		} else {
			resp.Status = "degraded"
		}
	} else {
		resp.Status = "degraded"
	}

	if resp.Database.Reachable && deps.Queries != nil {
		serverTime, err := deps.Queries.ServerTime(ctx)
		if err == nil && serverTime.Valid {
			formatted := serverTime.Time.UTC().Format(time.RFC3339Nano)
			resp.ServerTime = &formatted
		}
	}

	return resp
}

func handleHealth(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := buildHealthResponse(r.Context(), deps)
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "resource not found")
}

func handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}
