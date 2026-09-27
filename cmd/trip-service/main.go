package main

import (
	"context"
	"errors"
	"log"
	stdhttp "net/http"
	"os/signal"
	"syscall"

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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()

	tm := txmanager.New(pool)
	tripRepo := repository.NewTripRepository(pool, cfg.DatabaseQueryTimeout)
	historyRepo := repository.NewHistoryRepository(pool, cfg.DatabaseQueryTimeout)
	tripService := service.NewTripService(tm, tripRepo, historyRepo)
	tripHandler := handler.NewTripHandler(tripService, pool)

	router := chi.NewRouter()
	apiHandler := api.HandlerWithOptions(tripHandler, api.ChiServerOptions{
		BaseRouter:       router,
		ErrorHandlerFunc: handler.WriteRequestError,
	})

	server := &stdhttp.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           apiHandler,
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	go func() {
		log.Printf("listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
			log.Printf("server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}
}
