//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package positions

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type StaleTrip struct {
	TripID   uuid.UUID
	DriverID uuid.UUID
}

type (
	lister interface {
		ListStaleActive(ctx context.Context, staleAfter time.Duration, now time.Time) ([]StaleTrip, error)
	}
	pusher interface {
		RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error
	}
	metrics interface {
		StaleCheck(ctx context.Context, result string)
		QueueSizeAdd(ctx context.Context, delta int64)
		WorkersActiveAdd(ctx context.Context, delta int64)
		PositionRequestResult(ctx context.Context, result string, seconds float64)
	}
	logger interface {
		InfoCtx(ctx context.Context, msg string, args ...any)
		WarnCtx(ctx context.Context, msg string, args ...any)
		ErrorCtx(ctx context.Context, msg string, args ...any)
	}
	clock interface{ Now() time.Time }
)
