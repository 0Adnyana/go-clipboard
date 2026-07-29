package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/0adnyana/go-clipboard/internal/clips"
	"github.com/0adnyana/go-clipboard/internal/config"
	"github.com/0adnyana/go-clipboard/internal/database"
	"github.com/0adnyana/go-clipboard/internal/db"
	"github.com/0adnyana/go-clipboard/internal/httpapi"
	appmigrate "github.com/0adnyana/go-clipboard/internal/migrate"
)

const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			if err := runServe(logger); err != nil {
				logger.Error("server failed", slog.Any("error", err))
				os.Exit(1)
			}
			return
		case "migrate":
			if len(os.Args) < 3 {
				printUsage(os.Stderr)
				os.Exit(1)
			}
			if err := runMigrate(logger, os.Args[2]); err != nil {
				logger.Error("migrate failed", slog.Any("error", err))
				os.Exit(1)
			}
			return
		default:
			printUsage(os.Stderr)
			os.Exit(1)
		}
	}

	if err := runServe(logger); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}

func runServe(logger *slog.Logger) error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	pool.VerifyConnectivity(ctx, logger)

	// The migration runner is built once here rather than per request. A failure
	// to build it is not fatal: the health endpoint then reports the migration
	// state as unknown, which is the diagnostic a developer needs, whereas
	// refusing to start would leave them with no status page at all.
	var migrations httpapi.MigrationChecker
	runner, err := appmigrate.NewRunner(cfg.MigrationsDir, pool.Pool)
	if err != nil {
		logger.Error("migration checks unavailable", slog.Any("error", err))
	} else {
		defer runner.Close()
		migrations = runner
		logMigrationStatus(ctx, logger, runner)
	}

	queries := db.New(pool)
	clipSvc := clips.NewService(clips.NewPGStore(queries))

	server, err := httpapi.NewServer(logger, httpapi.Dependencies{
		Clips: clipSvc,
		Health: httpapi.HealthDependencies{
			Pool:       pool,
			Migrations: migrations,
			Queries:    queries,
		},
	})
	if err != nil {
		return err
	}
	addr := fmt.Sprintf(":%d", cfg.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: server.Handler(),
	}

	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", slog.String("addr", addr))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-runCtx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return err
	}

	logger.Info("server stopped")
	return nil
}

// logMigrationStatus runs one check during wiring. Beyond reporting the state at
// startup, it means goose's first call — which creates its version table — is
// issued here rather than inside a GET /api/health.
func logMigrationStatus(ctx context.Context, logger *slog.Logger, runner *appmigrate.Runner) {
	status, err := runner.CheckPending(ctx)
	if err != nil {
		logger.Error("migration check failed", slog.Any("error", err))
		return
	}
	logger.Info("migration status",
		slog.Bool("pending", status.Pending),
		slog.Int64("currentVersion", status.CurrentVersion),
	)
}

func runMigrate(logger *slog.Logger, command string) error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	runner, err := appmigrate.NewRunner(cfg.MigrationsDir, pool.Pool)
	if err != nil {
		return err
	}
	defer runner.Close()

	switch command {
	case "up":
		return runner.Up(ctx, logger)
	case "down":
		return runner.Down(ctx, logger)
	case "status":
		return runner.PrintStatus(ctx, logger)
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown migrate command: %s", command)
	}
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, "usage: server [serve|migrate <up|down|status>]")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  serve                 start the HTTP server (default when no command is given)")
	fmt.Fprintln(w, "  migrate up            apply pending migrations")
	fmt.Fprintln(w, "  migrate down          roll back the latest migration")
	fmt.Fprintln(w, "  migrate status        show migration status")
}
