package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type TripMetrics struct {
	created   metric.Int64Counter
	completed metric.Int64Counter
}

func NewTripMetrics(m metric.Meter) (*TripMetrics, error) {
	created, err := m.Int64Counter("tripgo.trips.created",
		metric.WithDescription("Number of created trips"))
	if err != nil {
		return nil, err
	}
	completed, err := m.Int64Counter("tripgo.trips.completed",
		metric.WithDescription("Number of completed trips"))
	if err != nil {
		return nil, err
	}
	return &TripMetrics{created: created, completed: completed}, nil
}

func (t *TripMetrics) TripCreated(ctx context.Context) {
	t.created.Add(ctx, 1)
}

func (t *TripMetrics) TripCompleted(ctx context.Context) {
	t.completed.Add(ctx, 1)
}

type HTTPInstruments struct {
	Duration metric.Float64Histogram
	Active   metric.Int64UpDownCounter
	Requests metric.Int64Counter
}

func NewHTTPInstruments(m metric.Meter) (*HTTPInstruments, error) {
	duration, err := m.Float64Histogram("http.server.request.duration",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	active, err := m.Int64UpDownCounter("http.server.active_requests",
		metric.WithDescription("Number of in-flight HTTP requests"))
	if err != nil {
		return nil, err
	}
	requests, err := m.Int64Counter("http.server.requests",
		metric.WithDescription("Total number of HTTP requests"))
	if err != nil {
		return nil, err
	}
	return &HTTPInstruments{Duration: duration, Active: active, Requests: requests}, nil
}

func (h *HTTPInstruments) ActiveAdd(ctx context.Context, v int64) { h.Active.Add(ctx, v) }

func (h *HTTPInstruments) RecordDuration(ctx context.Context, seconds float64, method, route string, status int) {
	h.Duration.Record(ctx, seconds, metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
		attribute.Int("http.response.status_code", status),
	))
}

func (h *HTTPInstruments) CountRequest(ctx context.Context, method, route string, status int) {
	h.Requests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("http.request.method", method),
		attribute.String("http.route", route),
		attribute.Int("http.response.status_code", status),
	))
}
