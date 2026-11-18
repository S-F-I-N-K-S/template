package model

import (
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
)

type SavePositionInput struct {
	TripID      uuid.UUID
	Coordinates domaintrip.Coordinates
	RecordedAt  time.Time
}
