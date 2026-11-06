package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/S-F-I-N-K-S/template/internal/config"
	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/repository"
	"github.com/S-F-I-N-K-S/template/internal/service"
	"github.com/S-F-I-N-K-S/template/internal/storage/postgres"
	handler "github.com/S-F-I-N-K-S/template/internal/transport/http"
	"github.com/S-F-I-N-K-S/template/internal/txmanager"
)

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()

	tm := txmanager.New(pool)
	tripRepo := repository.NewTripRepository(pool)
	historyRepo := repository.NewHistoryRepository(pool)
	tripService := service.NewTripService(tm, tripRepo, historyRepo)
	tripHandler := handler.NewTripHandler(tripService, pool, logger)

	router := chi.NewRouter()
	server := &stdhttp.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.HandlerFromMux(tripHandler, router),
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); !errors.Is(err, stdhttp.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		stop()
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("service stopped")
	return nil
}
