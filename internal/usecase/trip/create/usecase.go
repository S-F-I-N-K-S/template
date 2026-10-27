package create

import (
	"context"
	"errors"
	"fmt"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	"github.com/S-F-I-N-K-S/template/internal/model"
)

type UseCase struct {
	tx      txManager
	trips   tripRepository
	history historyRepository
	clk     clock
	gen     idGenerator
	m       metrics
	log     logger
}

func New(tx txManager, trips tripRepository, history historyRepository,
	clk clock, gen idGenerator, m metrics, log logger,
) *UseCase {
	return &UseCase{tx: tx, trips: trips, history: history, clk: clk, gen: gen, m: m, log: log}
}

func (u *UseCase) Execute(ctx context.Context, in model.CreateTripInput) (domaintrip.Trip, error) {
	t, err := domaintrip.New(domaintrip.NewParams{
		ID:         u.gen.New(),
		UserID:     in.UserID,
		DriverID:   in.DriverID,
		StartPoint: in.StartPoint,
		EndPoint:   in.EndPoint,
		Price:      in.Price,
		Now:        u.clk.Now(),
	})
	if err != nil {
		u.log.Info("trip create rejected by validation", "driver_id", in.DriverID, "err", err)
		return domaintrip.Trip{}, err
	}

	err = u.tx.Do(ctx, func(ctx context.Context) error {
		if err := u.trips.Save(ctx, t); err != nil {
			return err
		}
		return u.history.AppendHistory(ctx, t.ID, "", string(domaintrip.StatusActive), "trip created")
	})
	if err != nil {
		switch {
		case errors.Is(err, domaintrip.ErrDriverBusy):
			u.log.Info("trip create conflict: driver busy", "trip_id", t.ID, "driver_id", t.DriverID)
		default:
			u.log.Error("trip create failed", "trip_id", t.ID, "err", err)
		}
		return domaintrip.Trip{}, fmt.Errorf("create trip: %w", err)
	}

	u.m.TripCreated(ctx)
	u.log.Info("trip created", "trip_id", t.ID, "driver_id", t.DriverID, "user_id", t.UserID)
	return t, nil
}
