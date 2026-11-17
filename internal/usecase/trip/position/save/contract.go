//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package save

import (
	"context"
	"time"

	"github.com/google/uuid"

	domainposition "github.com/S-F-I-N-K-S/template/internal/domain/position"
)

type (
	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}
	tripRepository interface {
		AdvanceLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error
	}
	positionRepository interface {
		Save(ctx context.Context, p domainposition.Position) (domainposition.Position, error)
	}
	canceller interface {
		Cancel(tripID uuid.UUID)
	}
	clock   interface{ Now() time.Time }
	metrics interface{ PositionSaved(ctx context.Context) }
	logger  interface {
		InfoCtx(ctx context.Context, msg string, args ...any)
		ErrorCtx(ctx context.Context, msg string, args ...any)
	}
)
