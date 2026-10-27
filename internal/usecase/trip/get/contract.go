package get

import (
	"context"

	"github.com/google/uuid"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
)

type (
	tripReader interface {
		GetByID(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error)
	}
	logger interface {
		Info(msg string, args ...any)
		Error(msg string, args ...any)
	}
)
