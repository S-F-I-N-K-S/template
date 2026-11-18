package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const kindRequestPosition = "REQUEST_POSITION"

var (
	ErrUnavailable = errors.New("push service unavailable")
	ErrRejected    = errors.New("push service rejected request")
)

type pushRequest struct {
	RequestID   string            `json:"request_id"`
	RecipientID uuid.UUID         `json:"recipient_id"`
	Kind        string            `json:"kind"`
	Data        map[string]string `json:"data,omitempty"`
}

type Gateway struct {
	client  *http.Client
	baseURL string
	tracer  trace.Tracer
}

func New(baseURL string, timeout time.Duration, tracer trace.Tracer) *Gateway {
	if tracer == nil {
		tracer = noop.NewTracerProvider().Tracer("push")
	}
	return &Gateway{
		client: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
		tracer:  tracer,
	}
}

func (g *Gateway) RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error {
	ctx, span := g.tracer.Start(ctx, "push.request_position", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	span.SetAttributes(
		attribute.String("push.recipient_id", driverID.String()),
		attribute.String("push.trip_id", tripID.String()),
	)

	err := g.doRequest(ctx, driverID, tripID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

func (g *Gateway) doRequest(ctx context.Context, driverID, tripID uuid.UUID) error {
	body, err := json.Marshal(pushRequest{
		RequestID:   uuid.NewString(),
		RecipientID: driverID,
		Kind:        kindRequestPosition,
		Data:        map[string]string{"trip_id": tripID.String()},
	})
	if err != nil {
		return fmt.Errorf("marshal push request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/api/v1/push", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusAccepted:
		return nil
	case resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode >= http.StatusInternalServerError:
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	case resp.StatusCode >= http.StatusBadRequest:
		return fmt.Errorf("%w: status %d", ErrRejected, resp.StatusCode)
	default:
		return fmt.Errorf("push service: unexpected status %d", resp.StatusCode)
	}
}

func (g *Gateway) Close() {
	g.client.CloseIdleConnections()
}
