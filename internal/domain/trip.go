package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type TripStatus string

const (
	StatusActive    TripStatus = "active"
	StatusCompleted TripStatus = "completed"
)

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
	Status     TripStatus
	StartedAt  time.Time
	FinishedAt *time.Time
}

var (
	ErrDriverBusy    = errors.New("driver already has an active trip")
	ErrTripNotFound  = errors.New("trip not found")
	ErrTripCompleted = errors.New("trip already completed")
)
