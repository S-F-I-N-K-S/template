package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

type TripMetrics struct{ created, completed metric.Int64Counter }

func NewTripMetrics(m metric.Meter) (*TripMetrics, error) {
	panic("TODO: telemetry.NewTripMetrics")
}

func (t *TripMetrics) TripCreated(ctx context.Context) {
	panic("TODO: TripMetrics.TripCreated")
}

func (t *TripMetrics) TripCompleted(ctx context.Context) {
	panic("TODO: TripMetrics.TripCompleted")
}

func NewHTTPInstruments(m metric.Meter) (duration metric.Float64Histogram, active metric.Int64UpDownCounter, err error) {
	panic("TODO: telemetry.NewHTTPInstruments")
}
