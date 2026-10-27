package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	ServiceName, ServiceVersion string
	LogLevel                    slog.Level
}

type Telemetry struct {
	Logger         *Logger
	TracerProvider trace.TracerProvider
	MeterProvider  otelmetric.MeterProvider
	shutdown       func(context.Context) error
}

func Setup(ctx context.Context, cfg Config) (*Telemetry, error) {
	res, err := sdkresource.New(ctx, sdkresource.WithAttributes(
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
	))
	if err != nil {
		return nil, fmt.Errorf("telemetry resource: %w", err)
	}

	traceExp, err := newOTLPTraceExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	metricExp, err := newOTLPMetricExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(defaultMetricInterval))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	logExp, err := newOTLPLogExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("log exporter: %w", err)
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)
	otel.SetLoggerProvider(lp)

	logger := newLogger(cfg.LogLevel, lp, mp, cfg.ServiceName)

	shutdownAll := func(shutdownCtx context.Context) error {
		var errs []error
		if err := tp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("tracer provider shutdown: %w", err))
		}
		if err := mp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
		}
		if err := lp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("logger provider shutdown: %w", err))
		}
		return errors.Join(errs...)
	}

	return &Telemetry{
		Logger:         logger,
		TracerProvider: tp,
		MeterProvider:  mp,
		shutdown:       shutdownAll,
	}, nil
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	return t.shutdown(ctx)
}

const defaultMetricInterval = 15 * time.Second

func timeNow() time.Time { return time.Now().UTC() }
