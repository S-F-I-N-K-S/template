package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type PositionMetrics struct {
	saved metric.Int64Counter
}

func NewPositionMetrics(m metric.Meter) (*PositionMetrics, error) {
	saved, err := m.Int64Counter("tripgo.positions.saved",
		metric.WithDescription("Number of saved trip positions"))
	if err != nil {
		return nil, err
	}
	return &PositionMetrics{saved: saved}, nil
}

func (p *PositionMetrics) PositionSaved(ctx context.Context) {
	p.saved.Add(ctx, 1)
}

type WorkerMetrics struct {
	staleChecks   metric.Int64Counter
	queueSize     metric.Int64UpDownCounter
	workersActive metric.Int64UpDownCounter
	requests      metric.Int64Counter
	duration      metric.Float64Histogram
}

func NewWorkerMetrics(m metric.Meter) (*WorkerMetrics, error) {
	staleChecks, err := m.Int64Counter("tripgo.stale_checks",
		metric.WithDescription("Outcome of each stale trip queueing attempt: duplicate, queued or dropped"))
	if err != nil {
		return nil, err
	}
	queueSize, err := m.Int64UpDownCounter("tripgo.position_requests.queue.size",
		metric.WithDescription("Current number of position requests waiting in the queue"))
	if err != nil {
		return nil, err
	}
	workersActive, err := m.Int64UpDownCounter("tripgo.position_requests.workers.active",
		metric.WithDescription("Number of workers currently calling the Push Service"))
	if err != nil {
		return nil, err
	}
	requests, err := m.Int64Counter("tripgo.position_requests",
		metric.WithDescription("Outcome of each position request sent to the Push Service"))
	if err != nil {
		return nil, err
	}
	duration, err := m.Float64Histogram("tripgo.position_requests.duration",
		metric.WithDescription("Push Service request duration in seconds"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}
	return &WorkerMetrics{
		staleChecks:   staleChecks,
		queueSize:     queueSize,
		workersActive: workersActive,
		requests:      requests,
		duration:      duration,
	}, nil
}

func (w *WorkerMetrics) StaleCheck(ctx context.Context, result string) {
	w.staleChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

func (w *WorkerMetrics) QueueSizeAdd(ctx context.Context, delta int64) {
	w.queueSize.Add(ctx, delta)
}

func (w *WorkerMetrics) WorkersActiveAdd(ctx context.Context, delta int64) {
	w.workersActive.Add(ctx, delta)
}

func (w *WorkerMetrics) PositionRequestResult(ctx context.Context, result string, seconds float64) {
	w.requests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
	w.duration.Record(ctx, seconds, metric.WithAttributes(attribute.String("result", result)))
}
