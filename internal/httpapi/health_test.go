package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	appmigrate "github.com/0adnyana/go-clipboard/internal/migrate"
)

type fakeQuerier struct {
	serverTime pgtype.Timestamptz
	err        error
}

func (f fakeQuerier) ServerTime(ctx context.Context) (pgtype.Timestamptz, error) {
	return f.serverTime, f.err
}

type fakeMigrationChecker struct {
	status appmigrate.Status
	err    error
}

func (f fakeMigrationChecker) CheckPending(ctx context.Context) (appmigrate.Status, error) {
	return f.status, f.err
}

func TestBuildHealthResponse_omitsServerTimeWhenDatabaseUnreachable(t *testing.T) {
	when := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	resp := buildHealthResponse(context.Background(), Dependencies{
		Queries: fakeQuerier{
			serverTime: pgtype.Timestamptz{Time: when, Valid: true},
		},
	})

	if resp.ServerTime != nil {
		t.Fatalf("serverTime = %v, want nil when database is unreachable", resp.ServerTime)
	}
}

func TestBuildHealthResponse_withoutPoolIsDegraded(t *testing.T) {
	resp := buildHealthResponse(context.Background(), Dependencies{})
	if resp.Status != "degraded" {
		t.Fatalf("status = %q, want degraded", resp.Status)
	}
	if resp.Database.Reachable {
		t.Fatal("expected database unreachable")
	}
}

func TestBuildHealthResponse_omitsMigrationsWhenCheckCannotRun(t *testing.T) {
	resp := buildHealthResponse(context.Background(), Dependencies{})

	if resp.Migrations != nil {
		t.Fatalf("migrations = %+v, want nil when the check cannot run", *resp.Migrations)
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if bytes.Contains(encoded, []byte(`"migrations"`)) {
		t.Fatalf("body %s must not assert a migration state it never observed", encoded)
	}
}

func TestBuildHealthResponse_reportsPendingMigrationsFromTheChecker(t *testing.T) {
	resp := buildHealthResponse(context.Background(), Dependencies{
		Migrations: fakeMigrationChecker{
			status: appmigrate.Status{Pending: true, CurrentVersion: 20260726130213},
		},
	})

	if resp.Migrations == nil {
		t.Fatal("migrations = nil, want the state the checker reported")
	}
	if !resp.Migrations.Pending || resp.Migrations.CurrentVersion != 20260726130213 {
		t.Fatalf("migrations = %+v, want pending at version 20260726130213", *resp.Migrations)
	}
	if resp.Status != "degraded" {
		t.Fatalf("status = %q, want degraded while migrations are pending", resp.Status)
	}
}

func TestBuildHealthResponse_omitsMigrationsWhenTheCheckerFails(t *testing.T) {
	resp := buildHealthResponse(context.Background(), Dependencies{
		Migrations: fakeMigrationChecker{err: errors.New("database is unreachable")},
	})

	if resp.Migrations != nil {
		t.Fatalf("migrations = %+v, want nil when the check failed", *resp.Migrations)
	}
	if resp.Status != "degraded" {
		t.Fatalf("status = %q, want degraded", resp.Status)
	}
}

func TestFakeQuerier_returnsConfiguredTime(t *testing.T) {
	when := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	querier := fakeQuerier{
		serverTime: pgtype.Timestamptz{Time: when, Valid: true},
	}

	got, err := querier.ServerTime(context.Background())
	if err != nil {
		t.Fatalf("ServerTime() error = %v", err)
	}
	if !got.Valid || !got.Time.Equal(when) {
		t.Fatalf("ServerTime() = %+v, want %+v", got, when)
	}
}
