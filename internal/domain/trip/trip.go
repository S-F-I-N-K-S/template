package trip

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	latMin = -90.0
	latMax = 90.0
	lonMin = -180.0
	lonMax = 180.0

	reasonRequired  = "is required"
	reasonLatRange  = "out of range [-90, 90]"
	reasonLonRange  = "out of range [-180, 180]"
	reasonPriceNeg  = "must be >= 0"
	reasonTimeOrder = "must not be before started_at"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

var allowedTransitions = map[Status][]Status{
	StatusActive:    {StatusCompleted},
	StatusCompleted: {},
}

func (s Status) CanTransitionTo(to Status) bool {
	for _, allowed := range allowedTransitions[s] {
		if allowed == to {
			return true
		}
	}
	return false
}

type Coordinates struct {
	Latitude  float64
	Longitude float64
}

type Trip struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	DriverID   uuid.UUID
	StartPoint Coordinates
	EndPoint   Coordinates
	Price      int64
	Status     Status
	StartedAt  time.Time
	FinishedAt *time.Time
}

type NewParams struct {
	ID, UserID, DriverID uuid.UUID
	StartPoint, EndPoint Coordinates
	Price                int64
	Now                  time.Time
}

func New(p NewParams) (Trip, error) {
	t := Trip{
		ID:         p.ID,
		UserID:     p.UserID,
		DriverID:   p.DriverID,
		StartPoint: p.StartPoint,
		EndPoint:   p.EndPoint,
		Price:      p.Price,
		Status:     StatusActive,
		StartedAt:  p.Now.UTC(),
	}
	if err := t.Validate(); err != nil {
		return Trip{}, err
	}
	return t, nil
}

func (t *Trip) Finish(at time.Time) error {
	if !t.Status.CanTransitionTo(StatusCompleted) {
		return ErrTripCompleted
	}
	t.Status = StatusCompleted
	t.FinishedAt = &at
	return nil
}

func (t Trip) Validate() error {
	if t.UserID == uuid.Nil {
		return newValidation("user_id", reasonRequired)
	}
	if t.DriverID == uuid.Nil {
		return newValidation("driver_id", reasonRequired)
	}
	if err := t.StartPoint.Validate(); err != nil {
		return fmt.Errorf("start_point: %w", err)
	}
	if err := t.EndPoint.Validate(); err != nil {
		return fmt.Errorf("end_point: %w", err)
	}
	if t.Price < 0 {
		return newValidation("price", reasonPriceNeg)
	}
	if t.FinishedAt != nil && t.FinishedAt.Before(t.StartedAt) {
		return newValidation("finished_at", reasonTimeOrder)
	}
	return nil
}

func (c Coordinates) Validate() error {
	if c.Latitude < latMin || c.Latitude > latMax {
		return newValidation("latitude", reasonLatRange)
	}
	if c.Longitude < lonMin || c.Longitude > lonMax {
		return newValidation("longitude", reasonLonRange)
	}
	return nil
}

func AsValidation(err error) (*ValidationError, bool) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve, true
	}
	return nil, false
}
