package list

import (
	"context"

	"github.com/google/uuid"

	domainposition "github.com/S-F-I-N-K-S/template/internal/domain/position"
)

type usecase interface {
	Execute(ctx context.Context, tripID uuid.UUID) ([]domainposition.Position, error)
}
