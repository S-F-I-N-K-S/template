//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package create

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
		Save(ctx context.Context, t domaintrip.Trip) error
	}
	historyRepository interface {
		AppendHistory(ctx context.Context, tripID uuid.UUID, from, to, reason string) error
	}
	clock   interface{ Now() time.Time }
	metrics interface{ TripCreated(ctx context.Context) }
)
