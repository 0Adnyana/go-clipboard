package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

type Status struct {
	Pending        bool
	CurrentVersion int64
}

// Runner owns a goose provider together with the database/sql handle adapted
// from the pgx pool. Wiring code creates one and keeps it for the process
// lifetime: building a provider per call would open a new *sql.DB on every
// health request, and goose issues the DDL creating its version table on the
// first call against a provider, so the cost is not only allocation.
type Runner struct {
	provider *goose.Provider
	db       *sql.DB
}

func NewRunner(migrationsDir string, pool *pgxpool.Pool) (*Runner, error) {
	if err := MigrationsDirExists(migrationsDir); err != nil {
		return nil, err
	}

	db := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS(migrationsDir))
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	return &Runner{provider: provider, db: db}, nil
}

// Close releases the database/sql handle. It deliberately leaves the pgx pool
// alone: OpenDBFromPool does not own the pool, which main closes explicitly and
// last.
func (r *Runner) Close() error {
	return r.db.Close()
}

func (r *Runner) Up(ctx context.Context, logger *slog.Logger) error {
	results, err := r.provider.Up(ctx)
	if err != nil {
		return err
	}
	for _, result := range results {
		logger.Info("migration applied", slog.String("source", result.Source.Path), slog.Int64("version", result.Source.Version))
	}
	return nil
}

func (r *Runner) Down(ctx context.Context, logger *slog.Logger) error {
	result, err := r.provider.Down(ctx)
	if err != nil {
		return err
	}
	if result != nil {
		logger.Info("migration rolled back", slog.String("source", result.Source.Path), slog.Int64("version", result.Source.Version))
	}
	return nil
}

func (r *Runner) PrintStatus(ctx context.Context, logger *slog.Logger) error {
	statuses, err := r.provider.Status(ctx)
	if err != nil {
		return err
	}
	for _, status := range statuses {
		state := "pending"
		if status.State == goose.StateApplied {
			state = "applied"
		}
		logger.Info("migration status",
			slog.String("source", status.Source.Path),
			slog.Int64("version", status.Source.Version),
			slog.String("state", state),
		)
	}
	version, err := r.provider.GetDBVersion(ctx)
	if err != nil {
		return err
	}
	logger.Info("current migration version", slog.Int64("version", version))
	return nil
}

// CheckPending reports whether unapplied migrations exist without applying any:
// goose's HasPending skips session locking for exactly this use.
func (r *Runner) CheckPending(ctx context.Context) (Status, error) {
	pending, err := r.provider.HasPending(ctx)
	if err != nil {
		return Status{}, err
	}

	version, err := r.provider.GetDBVersion(ctx)
	if err != nil {
		return Status{}, err
	}

	return Status{
		Pending:        pending,
		CurrentVersion: version,
	}, nil
}

func MigrationsDirExists(migrationsDir string) error {
	info, err := os.Stat(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("migrations directory not found: %s", migrationsDir)
		}
		return fmt.Errorf("read migrations directory %s: %w", migrationsDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("migrations path is not a directory: %s", migrationsDir)
	}
	return nil
}
