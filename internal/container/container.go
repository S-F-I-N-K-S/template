package container

import (
	"context"
	"errors"
	stdhttp "net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/S-F-I-N-K-S/template/internal/config"
	pushgw "github.com/S-F-I-N-K-S/template/internal/gateway/push"
	httpapi "github.com/S-F-I-N-K-S/template/internal/http"
	"github.com/S-F-I-N-K-S/template/internal/http/domainerror"
	"github.com/S-F-I-N-K-S/template/internal/http/health"
	"github.com/S-F-I-N-K-S/template/internal/http/trip/create"
	chandlers "github.com/S-F-I-N-K-S/template/internal/http/trip/finish"
	gethandler "github.com/S-F-I-N-K-S/template/internal/http/trip/get"
	positionlisthandler "github.com/S-F-I-N-K-S/template/internal/http/trip/position/list"
	positionsavehandler "github.com/S-F-I-N-K-S/template/internal/http/trip/position/save"
	"github.com/S-F-I-N-K-S/template/internal/infra/postgres"
	"github.com/S-F-I-N-K-S/template/internal/infra/telemetry"
	"github.com/S-F-I-N-K-S/template/internal/infra/transaction"
	repoposition "github.com/S-F-I-N-K-S/template/internal/repository/position"
	repostrip "github.com/S-F-I-N-K-S/template/internal/repository/trip"
	ucCreate "github.com/S-F-I-N-K-S/template/internal/usecase/trip/create"
	ucFinish "github.com/S-F-I-N-K-S/template/internal/usecase/trip/finish"
	ucGet "github.com/S-F-I-N-K-S/template/internal/usecase/trip/get"
	ucPositionList "github.com/S-F-I-N-K-S/template/internal/usecase/trip/position/list"
	ucPositionSave "github.com/S-F-I-N-K-S/template/internal/usecase/trip/position/save"
	workerpositions "github.com/S-F-I-N-K-S/template/internal/worker/positions"
)

type Container struct {
	server     *stdhttp.Server
	pool       *pgxpool.Pool
	telemetry  *telemetry.Telemetry
	workers    *workerpositions.Pool
	pushClient *pushgw.Gateway
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

	pool, err := postgres.NewPool(ctx, cfg, tm.TracerProvider, tm.MeterProvider)
	if err != nil {
		_ = tm.Shutdown(context.Background())
		return nil, err
	}

	txm := transaction.New(pool)
	trips := repostrip.NewRepository(pool, cfg.DatabaseQueryTimeout)
	history := repostrip.NewHistoryRepository(pool, cfg.DatabaseQueryTimeout)
	positionsRepo := repoposition.NewRepository(pool, cfg.DatabaseQueryTimeout)

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
	positionMetrics, err := telemetry.NewPositionMetrics(meter)
	if err != nil {
		_ = tm.Shutdown(context.Background())
		pool.Close()
		return nil, err
	}
	workerMetrics, err := telemetry.NewWorkerMetrics(meter)
	if err != nil {
		_ = tm.Shutdown(context.Background())
		pool.Close()
		return nil, err
	}

	errs := domainerror.NewWriter(log)

	pushClient := pushgw.New(cfg.PushHTTPURL, cfg.PushRequestTimeout, tm.TracerProvider.Tracer("tripgo/gateway/push"))

	workerPool := workerpositions.New(
		workerpositions.Config{
			CheckInterval:  cfg.PositionCheckInterval,
			StaleAfter:     cfg.PositionStaleAfter,
			Workers:        cfg.PositionWorkers,
			QueueSize:      cfg.PositionQueueSize,
			RequestTimeout: cfg.PushRequestTimeout,
		},
		staleTripLister{trips: trips},
		pushClient,
		workerMetrics,
		log,
		systemClock{},
		workerpositions.NewTicker(cfg.PositionCheckInterval),
		tm.TracerProvider.Tracer("tripgo/worker/positions"),
	)

	createUC := ucCreate.New(txm, trips, history, systemClock{}, uuidGen{}, tripMetrics, log)
	getUC := ucGet.New(trips, log)
	finishUC := ucFinish.New(txm, trips, history, systemClock{}, tripMetrics, log)
	savePositionUC := ucPositionSave.New(txm, trips, positionsRepo, workerPool, systemClock{}, positionMetrics, log)
	listPositionUC := ucPositionList.New(trips, positionsRepo, log)

	router := httpapi.NewRouter(httpapi.Deps{
		Create:       create.New(createUC, errs),
		Get:          gethandler.New(getUC, errs),
		Finish:       chandlers.New(finishUC, errs),
		SavePosition: positionsavehandler.New(savePositionUC, errs),
		ListPosition: positionlisthandler.New(listPositionUC, errs),
		Health:       health.New(poolPinger{pool: pool}),
		Errs:         errs,
		Logger:       log,
		Metrics:      httpMetrics,
		Tracer:       tm.TracerProvider,
	})

	workerPool.Start()

	return &Container{
		server: &stdhttp.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           router,
			ReadTimeout:       cfg.HTTPReadTimeout,
			ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
			WriteTimeout:      cfg.HTTPWriteTimeout,
			IdleTimeout:       cfg.HTTPIdleTimeout,
		},
		pool:       pool,
		telemetry:  tm,
		workers:    workerPool,
		pushClient: pushClient,
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
	if err := c.workers.Stop(ctx); err != nil {
		errs = append(errs, err)
	}
	c.pushClient.Close()
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

type staleTripLister struct {
	trips *repostrip.Repository
}

func (s staleTripLister) ListStaleActive(
	ctx context.Context, staleAfter time.Duration, now time.Time,
) ([]workerpositions.StaleTrip, error) {
	rows, err := s.trips.ListStaleActive(ctx, staleAfter, now)
	if err != nil {
		return nil, err
	}
	out := make([]workerpositions.StaleTrip, 0, len(rows))
	for _, r := range rows {
		out = append(out, workerpositions.StaleTrip{TripID: r.TripID, DriverID: r.DriverID})
	}
	return out, nil
}
