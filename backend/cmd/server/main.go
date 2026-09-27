// @title           DataDeck API
// @version         1.0
// @description     Local-first database GUI backend. All responses use the standard envelope (success, data, error, meta).
// @BasePath        /api/v1
// @schemes         http
// @host            127.0.0.1:8080
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

	"github.com/datadeck/datadeck/backend/internal/api"
	"github.com/datadeck/datadeck/backend/internal/config"
	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/repository"
	"github.com/datadeck/datadeck/backend/internal/security"
	"github.com/datadeck/datadeck/backend/internal/storage"
	"github.com/datadeck/datadeck/backend/internal/version"
)

const shutdownTimeout = 10 * time.Second

func main() {
	// A hand-rolled check avoids a CLI framework for a single flag.
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-version" {
			fmt.Println(version.String())
			return
		}
	}
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "datadeck: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))

	credentials, err := security.NewCipher(cfg.EncryptionKey)
	if err != nil {
		return fmt.Errorf("credential cipher: %w", err)
	}
	logger.Info("security_ready")

	connections := database.NewManager(database.DefaultOptions(), database.Postgres{}, database.MySQL{}, database.SQLite{})
	defer func() {
		if err := connections.CloseAll(); err != nil {
			logger.Error("connection_manager_close_failed", slog.String("error", err.Error()))
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := storage.Open(ctx, cfg.StoragePath)
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("storage_close_failed", slog.String("error", err.Error()))
		}
	}()
	logger.Info("storage_ready", slog.String("path", store.Path()))

	connectionRepo := repository.NewConnectionRepository(store.DB())
	historyRepo := repository.NewQueryHistoryRepository(store.DB())
	savedQueryRepo := repository.NewSavedQueryRepository(store.DB())

	server := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           api.NewServer(cfg, logger, credentials, connectionRepo, historyRepo, savedQueryRepo, connections).Router(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server_starting",
			slog.String("addr", cfg.Addr()),
			slog.String("environment", string(cfg.Environment)),
			slog.Bool("loopback", cfg.IsLoopback()),
		)
		if !cfg.IsLoopback() {
			logger.Warn("remote_binding_enabled",
				slog.String("addr", cfg.Addr()),
				slog.String("note", "DataDeck has no authentication; the API is reachable from the network"),
			)
		}
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown_started")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	logger.Info("shutdown_complete")
	return nil
}
