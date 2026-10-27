//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
)

type (
	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}
	tripRepository interface {
		GetByID(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error)
		Finish(ctx context.Context, id uuid.UUID, finishedAt time.Time) error
	}
	historyRepository interface {
		AppendHistory(ctx context.Context, tripID uuid.UUID, from, to, reason string) error
	}
	clock   interface{ Now() time.Time }
	metrics interface{ TripCompleted(ctx context.Context) }
	logger  interface {
		Info(msg string, args ...any)
		Error(msg string, args ...any)
	}
)
