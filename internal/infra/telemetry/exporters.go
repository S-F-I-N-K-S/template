package telemetry

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func newOTLPTraceExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	return otlptracehttp.New(ctx)
}

func newOTLPMetricExporter(ctx context.Context) (*otlpmetrichttp.Exporter, error) {
	return otlpmetrichttp.New(ctx)
}

func newOTLPLogExporter(ctx context.Context) (sdklog.Exporter, error) {
	return otlploghttp.New(ctx)
}

func newStdoutLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
