package get

import (
	"context"

	"github.com/google/uuid"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
)

type usecase interface {
	Execute(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error)
}
