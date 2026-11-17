package position

import (
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
)

const maxFutureSkew = time.Minute

const (
	reasonRequired = "is required"
	reasonFuture   = "must not be more than 1 minute in the future"
)

type Position struct {
	ID          int64
	TripID      uuid.UUID
	Coordinates domaintrip.Coordinates
	RecordedAt  time.Time
}

type NewParams struct {
	TripID      uuid.UUID
	Coordinates domaintrip.Coordinates
	RecordedAt  time.Time
	Now         time.Time
}

func New(p NewParams) (Position, error) {
	pos := Position{
		TripID:      p.TripID,
		Coordinates: p.Coordinates,
		RecordedAt:  p.RecordedAt.UTC(),
	}
	if err := pos.validate(p.Now); err != nil {
		return Position{}, err
	}
	return pos, nil
}

func (p Position) validate(now time.Time) error {
	if p.RecordedAt.IsZero() {
		return &domaintrip.ValidationError{Field: "recorded_at", Reason: reasonRequired}
	}
	if p.RecordedAt.After(now.Add(maxFutureSkew)) {
		return &domaintrip.ValidationError{Field: "recorded_at", Reason: reasonFuture}
	}
	return p.Coordinates.Validate()
}
