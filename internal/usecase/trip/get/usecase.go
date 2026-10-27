package get

import (
	"context"
	"errors"

	"github.com/google/uuid"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
)

type UseCase struct {
	trips tripReader
	log   logger
}

func New(trips tripReader, log logger) *UseCase {
	return &UseCase{trips: trips, log: log}
}

func (u *UseCase) Execute(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error) {
	t, err := u.trips.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domaintrip.ErrTripNotFound) {
			u.log.Info("trip not found", "trip_id", id)
			return domaintrip.Trip{}, err
		}
		u.log.Error("trip read failed", "trip_id", id, "err", err)
		return domaintrip.Trip{}, err
	}
	return t, nil
}
