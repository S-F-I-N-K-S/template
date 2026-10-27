package container

import (
	"context"
	"errors"
	stdhttp "net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/S-F-I-N-K-S/template/internal/config"
	httpapi "github.com/S-F-I-N-K-S/template/internal/http"
	"github.com/S-F-I-N-K-S/template/internal/http/domainerror"
	"github.com/S-F-I-N-K-S/template/internal/http/health"
	"github.com/S-F-I-N-K-S/template/internal/http/trip/create"
	chandlers "github.com/S-F-I-N-K-S/template/internal/http/trip/finish"
	gethandler "github.com/S-F-I-N-K-S/template/internal/http/trip/get"
	"github.com/S-F-I-N-K-S/template/internal/infra/postgres"
	"github.com/S-F-I-N-K-S/template/internal/infra/telemetry"
	"github.com/S-F-I-N-K-S/template/internal/infra/transaction"
	repostrip "github.com/S-F-I-N-K-S/template/internal/repository/trip"
	ucCreate "github.com/S-F-I-N-K-S/template/internal/usecase/trip/create"
	ucFinish "github.com/S-F-I-N-K-S/template/internal/usecase/trip/finish"
	ucGet "github.com/S-F-I-N-K-S/template/internal/usecase/trip/get"
)

type Container struct {
	server    *stdhttp.Server
	pool      *pgxpool.Pool
	telemetry *telemetry.Telemetry
}

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	tm, err := telemetry.Setup(ctx, telemetry.Config{
		ServiceName:    "tripgo",
		ServiceVersion: cfg.ServiceVersion,
		LogLevel:       cfg.LogLevel,
	})
	if err != nil {
		return nil, err
	}

	log := tm.Logger
	log.Info("starting service", "addr", cfg.HTTPAddr, "version", cfg.ServiceVersion)

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		_ = tm.Shutdown(context.Background())
		return nil, err
	}

	txm := transaction.New(pool)
	trips := repostrip.NewRepository(pool, cfg.DatabaseQueryTimeout)
	history := repostrip.NewHistoryRepository(pool, cfg.DatabaseQueryTimeout)

	meter := tm.MeterProvider.Meter("tripgo")
	tripMetrics, err := telemetry.NewTripMetrics(meter)
	if err != nil {
		_ = tm.Shutdown(context.Background())
		pool.Close()
		return nil, err
	}
	httpMetrics, err := telemetry.NewHTTPInstruments(meter)
	if err != nil {
		_ = tm.Shutdown(context.Background())
		pool.Close()
		return nil, err
	}

	errs := domainerror.NewWriter(log)

	createUC := ucCreate.New(txm, trips, history, systemClock{}, uuidGen{}, tripMetrics, log)
	getUC := ucGet.New(trips, log)
	finishUC := ucFinish.New(txm, trips, history, systemClock{}, tripMetrics, log)

	router := httpapi.NewRouter(httpapi.Deps{
		Create:  create.New(createUC, errs),
		Get:     gethandler.New(getUC, errs),
		Finish:  chandlers.New(finishUC, errs),
		Health:  health.New(poolPinger{pool: pool}),
		Errs:    errs,
		Logger:  log,
		Metrics: httpMetrics,
		Tracer:  tm.TracerProvider,
	})

	return &Container{
		server: &stdhttp.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           router,
			ReadTimeout:       cfg.HTTPReadTimeout,
			ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
			WriteTimeout:      cfg.HTTPWriteTimeout,
			IdleTimeout:       cfg.HTTPIdleTimeout,
		},
		pool:      pool,
		telemetry: tm,
	}, nil
}

func (c *Container) Run() error {
	if err := c.server.ListenAndServe(); !errors.Is(err, stdhttp.ErrServerClosed) {
		return err
	}
	return nil
}

func (c *Container) Shutdown(ctx context.Context) error {
	var errs []error
	if err := c.server.Shutdown(ctx); err != nil {
		errs = append(errs, err)
	}
	c.pool.Close()
	if err := c.telemetry.Shutdown(ctx); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type uuidGen struct{}

func (uuidGen) New() uuid.UUID { return uuid.New() }

type poolPinger struct {
	pool interface {
		Ping(ctx context.Context) error
	}
}

func (p poolPinger) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }
