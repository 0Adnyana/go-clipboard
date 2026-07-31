package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/0adnyana/go-clipboard/internal/database"
	appmigrate "github.com/0adnyana/go-clipboard/internal/migrate"
)

type fakeServerClock struct {
	serverTime time.Time
	err        error
}

func (f fakeServerClock) ServerTime(ctx context.Context) (time.Time, error) {
	return f.serverTime, f.err
}

type fakeMigrationChecker struct {
	status appmigrate.Status
	err    error
}

func (f fakeMigrationChecker) CheckPending(ctx context.Context) (appmigrate.Status, error) {
	return f.status, f.err
}

type fakeDatabaseProber struct {
	result database.ProbeResult
}

func (f fakeDatabaseProber) Probe(ctx context.Context) database.ProbeResult {
	return f.result
}

func reachableHealthDeps() HealthDependencies {
	return HealthDependencies{
		Pool: fakeDatabaseProber{
			result: database.ProbeResult{Reachable: true, Latency: time.Millisecond},
		},
		Migrations: fakeMigrationChecker{
			status: appmigrate.Status{CurrentVersion: 1},
		},
	}
}

func TestBuildHealthResponse_degradesWhenServerTimeNil(t *testing.T) {
	resp := buildHealthResponse(context.Background(), reachableHealthDeps())

	if resp.Status != "degraded" {
		t.Fatalf("status = %q, want degraded when server time is unavailable", resp.Status)
	}
	if resp.ServerTime != nil {
		t.Fatalf("serverTime = %v, want nil when server time is unavailable", resp.ServerTime)
	}
}

func TestBuildHealthResponse_degradesWhenServerTimeErrors(t *testing.T) {
	deps := reachableHealthDeps()
	deps.ServerTime = fakeServerClock{err: errors.New("clock unavailable")}

	resp := buildHealthResponse(context.Background(), deps)

	if resp.Status != "degraded" {
		t.Fatalf("status = %q, want degraded when server time check fails", resp.Status)
	}
	if resp.ServerTime != nil {
		t.Fatalf("serverTime = %v, want nil when server time check fails", resp.ServerTime)
	}
}

func TestBuildHealthResponse_degradesWhenServerTimeZero(t *testing.T) {
	deps := reachableHealthDeps()
	deps.ServerTime = fakeServerClock{}

	resp := buildHealthResponse(context.Background(), deps)

	if resp.Status != "degraded" {
		t.Fatalf("status = %q, want degraded when server time is zero", resp.Status)
	}
	if resp.ServerTime != nil {
		t.Fatalf("serverTime = %v, want nil when server time is zero", resp.ServerTime)
	}
}

func TestBuildHealthResponse_includesServerTimeWhenValid(t *testing.T) {
	when := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	deps := reachableHealthDeps()
	deps.ServerTime = fakeServerClock{serverTime: when}

	resp := buildHealthResponse(context.Background(), deps)

	if resp.Status != "ok" {
		t.Fatalf("status = %q, want ok when server time is valid", resp.Status)
	}
	if resp.ServerTime == nil {
		t.Fatal("serverTime = nil, want RFC3339Nano UTC timestamp")
	}
	want := when.Format(time.RFC3339Nano)
	if *resp.ServerTime != want {
		t.Fatalf("serverTime = %q, want %q", *resp.ServerTime, want)
	}
}

func TestBuildHealthResponse_omitsServerTimeWhenDatabaseUnreachable(t *testing.T) {
	when := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	resp := buildHealthResponse(context.Background(), HealthDependencies{
		ServerTime: fakeServerClock{serverTime: when},
	})

	if resp.ServerTime != nil {
		t.Fatalf("serverTime = %v, want nil when database is unreachable", resp.ServerTime)
	}
}

func TestBuildHealthResponse_withoutPoolIsDegraded(t *testing.T) {
	resp := buildHealthResponse(context.Background(), HealthDependencies{})
	if resp.Status != "degraded" {
		t.Fatalf("status = %q, want degraded", resp.Status)
	}
	if resp.Database.Reachable {
		t.Fatal("expected database unreachable")
	}
}

func TestBuildHealthResponse_omitsMigrationsWhenCheckCannotRun(t *testing.T) {
	resp := buildHealthResponse(context.Background(), HealthDependencies{})

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
	resp := buildHealthResponse(context.Background(), HealthDependencies{
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
	resp := buildHealthResponse(context.Background(), HealthDependencies{
		Migrations: fakeMigrationChecker{err: errors.New("database is unreachable")},
	})

	if resp.Migrations != nil {
		t.Fatalf("migrations = %+v, want nil when the check failed", *resp.Migrations)
	}
	if resp.Status != "degraded" {
		t.Fatalf("status = %q, want degraded", resp.Status)
	}
}
