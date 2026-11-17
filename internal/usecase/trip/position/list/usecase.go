package list

import (
	"context"
	"errors"

	"github.com/google/uuid"

	domainposition "github.com/S-F-I-N-K-S/template/internal/domain/position"
	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
)

type UseCase struct {
	trips     tripReader
	positions positionReader
	log       logger
}

func New(trips tripReader, positions positionReader, log logger) *UseCase {
	return &UseCase{trips: trips, positions: positions, log: log}
}

func (u *UseCase) Execute(ctx context.Context, tripID uuid.UUID) ([]domainposition.Position, error) {
	if _, err := u.trips.GetByID(ctx, tripID); err != nil {
		if errors.Is(err, domaintrip.ErrTripNotFound) {
			u.log.InfoCtx(ctx, "list positions: trip not found", "trip_id", tripID)
			return nil, err
		}
		u.log.ErrorCtx(ctx, "list positions: trip read failed", "trip_id", tripID, "err", err)
		return nil, err
	}

	positions, err := u.positions.ListByTrip(ctx, tripID)
	if err != nil {
		u.log.ErrorCtx(ctx, "list positions failed", "trip_id", tripID, "err", err)
		return nil, err
	}
	return positions, nil
}
