//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package create

import (
	"context"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	"github.com/S-F-I-N-K-S/template/internal/model"
)

type usecase interface {
	Execute(ctx context.Context, in model.CreateTripInput) (domaintrip.Trip, error)
}
