package save

import (
	"context"

	domainposition "github.com/S-F-I-N-K-S/template/internal/domain/position"
	"github.com/S-F-I-N-K-S/template/internal/model"
)

type usecase interface {
	Execute(ctx context.Context, in model.SavePositionInput) (domainposition.Position, error)
}
