package model

import (
	"github.com/google/uuid"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
)

type CreateTripInput struct {
	UserID, DriverID     uuid.UUID
	StartPoint, EndPoint domaintrip.Coordinates
	Price                int64
}
