package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type Config struct {
	ServiceName, ServiceVersion string
	LogLevel                    slog.Level
}

type Telemetry struct {
	Logger         *slog.Logger
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	shutdown       func(context.Context) error
}

func Setup(ctx context.Context, cfg Config) (*Telemetry, error) {
	panic("TODO: telemetry.Setup")
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	return t.shutdown(ctx)
}
