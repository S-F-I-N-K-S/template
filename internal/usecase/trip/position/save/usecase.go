package save

import (
	"context"
	"errors"
	"fmt"

	domainposition "github.com/S-F-I-N-K-S/template/internal/domain/position"
	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	"github.com/S-F-I-N-K-S/template/internal/model"
)

type UseCase struct {
	tx        txManager
	trips     tripRepository
	positions positionRepository
	cancel    canceller
	clk       clock
	m         metrics
	log       logger
}

func New(tx txManager, trips tripRepository, positions positionRepository,
	cancel canceller, clk clock, m metrics, log logger,
) *UseCase {
	return &UseCase{tx: tx, trips: trips, positions: positions, cancel: cancel, clk: clk, m: m, log: log}
}

func (u *UseCase) Execute(ctx context.Context, in model.SavePositionInput) (domainposition.Position, error) {
	now := u.clk.Now()
	p, err := domainposition.New(domainposition.NewParams{
		TripID:      in.TripID,
		Coordinates: in.Coordinates,
		RecordedAt:  in.RecordedAt,
		Now:         now,
	})
	if err != nil {
		u.log.InfoCtx(ctx, "position save rejected by validation", "trip_id", in.TripID, "err", err)
		return domainposition.Position{}, err
	}

	var saved domainposition.Position
	err = u.tx.Do(ctx, func(ctx context.Context) error {
		if err := u.trips.AdvanceLastPositionAt(ctx, p.TripID, p.RecordedAt); err != nil {
			return err
		}
		s, err := u.positions.Save(ctx, p)
		if err != nil {
			return err
		}
		saved = s
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, domaintrip.ErrTripNotFound):
			u.log.InfoCtx(ctx, "position save: trip not found", "trip_id", in.TripID)
		case errors.Is(err, domaintrip.ErrTripCompleted):
			u.log.InfoCtx(ctx, "position save conflict: trip completed", "trip_id", in.TripID)
		default:
			u.log.ErrorCtx(ctx, "position save failed", "trip_id", in.TripID, "err", err)
		}
		return domainposition.Position{}, fmt.Errorf("save position: %w", err)
	}

	u.cancel.Cancel(p.TripID)

	u.m.PositionSaved(ctx)
	u.log.InfoCtx(ctx, "position saved", "trip_id", saved.TripID, "position_id", saved.ID)
	return saved, nil
}
