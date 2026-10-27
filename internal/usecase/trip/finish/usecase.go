package finish

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
)

type UseCase struct {
	tx      txManager
	trips   tripRepository
	history historyRepository
	clk     clock
	m       metrics
	log     logger
}

func New(tx txManager, trips tripRepository, history historyRepository, clk clock, m metrics, log logger) *UseCase {
	return &UseCase{tx: tx, trips: trips, history: history, clk: clk, m: m, log: log}
}

func (u *UseCase) Execute(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error) {
	finishedAt := u.clk.Now().UTC()

	var completed domaintrip.Trip
	err := u.tx.Do(ctx, func(ctx context.Context) error {
		t, err := u.trips.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if err := t.Finish(finishedAt); err != nil {
			return err
		}
		if err := u.trips.Finish(ctx, id, finishedAt); err != nil {
			return err
		}
		if err := u.history.AppendHistory(ctx, id, string(domaintrip.StatusActive), string(t.Status), "trip finished"); err != nil {
			return err
		}
		completed = t
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, domaintrip.ErrTripNotFound):
			u.log.Info("trip finish: not found", "trip_id", id)
		case errors.Is(err, domaintrip.ErrTripCompleted):
			u.log.Info("trip finish conflict: already completed", "trip_id", id)
		default:
			u.log.Error("trip finish failed", "trip_id", id, "err", err)
		}
		return domaintrip.Trip{}, fmt.Errorf("finish trip: %w", err)
	}

	u.m.TripCompleted(ctx)
	u.log.Info("trip completed", "trip_id", completed.ID, "driver_id", completed.DriverID)
	return completed, nil
}
