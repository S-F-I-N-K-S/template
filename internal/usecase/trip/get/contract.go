//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
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
		InfoCtx(ctx context.Context, msg string, args ...any)
		ErrorCtx(ctx context.Context, msg string, args ...any)
	}
)
