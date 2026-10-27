package finish_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	. "github.com/S-F-I-N-K-S/template/internal/usecase/trip/finish"
)

type deps struct {
	tx      *MocktxManager
	trips   *MocktripRepository
	history *MockhistoryRepository
	clock   *Mockclock
	metrics *Mockmetrics
	log     *Mocklogger
}

func newDeps(t *testing.T) (*gomock.Controller, deps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	return ctrl, deps{
		tx:      NewMocktxManager(ctrl),
		trips:   NewMocktripRepository(ctrl),
		history: NewMockhistoryRepository(ctrl),
		clock:   NewMockclock(ctrl),
		metrics: NewMockmetrics(ctrl),
		log:     NewMocklogger(ctrl),
	}
}

func passThrough(tx *MocktxManager) {
	tx.EXPECT().Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		Times(1)
}

var now = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func activeTrip(id uuid.UUID) domaintrip.Trip {
	return domaintrip.Trip{
		ID:         id,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: domaintrip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		EndPoint:   domaintrip.Coordinates{Latitude: 59.94, Longitude: 30.31},
		Price:      100,
		Status:     domaintrip.StatusActive,
		StartedAt:  now.Add(-time.Hour),
	}
}

func TestExecute_Success(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	id := uuid.New()
	tr := activeTrip(id)

	d.clock.EXPECT().Now().Return(now)
	passThrough(d.tx)
	d.trips.EXPECT().GetByID(gomock.Any(), id).Return(tr, nil)
	d.trips.EXPECT().Finish(gomock.Any(), id, now).Return(nil)
	d.history.EXPECT().AppendHistory(gomock.Any(), id, string(domaintrip.StatusActive), string(domaintrip.StatusCompleted), "trip finished").Return(nil)
	d.metrics.EXPECT().TripCompleted(gomock.Any())
	d.log.EXPECT().Info("trip completed", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	uc := New(d.tx, d.trips, d.history, d.clock, d.metrics, d.log)
	got, err := uc.Execute(context.Background(), id)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.Status != domaintrip.StatusCompleted || got.FinishedAt == nil {
		t.Errorf("trip = %+v", got)
	}
}

func TestExecute_NotFound(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	id := uuid.New()
	d.clock.EXPECT().Now().Return(now)
	passThrough(d.tx)
	d.trips.EXPECT().GetByID(gomock.Any(), id).Return(domaintrip.Trip{}, domaintrip.ErrTripNotFound)
	d.log.EXPECT().Info("trip finish: not found", gomock.Any(), gomock.Any())

	uc := New(d.tx, d.trips, d.history, d.clock, d.metrics, d.log)
	_, err := uc.Execute(context.Background(), id)
	if !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("err = %v, want ErrTripNotFound", err)
	}
}

func TestExecute_AlreadyCompleted(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	id := uuid.New()
	tr := activeTrip(id)
	tr.Status = domaintrip.StatusCompleted
	fin := now.Add(-time.Minute)
	tr.FinishedAt = &fin

	d.clock.EXPECT().Now().Return(now)
	passThrough(d.tx)
	d.trips.EXPECT().GetByID(gomock.Any(), id).Return(tr, nil)
	d.log.EXPECT().Info("trip finish conflict: already completed", gomock.Any(), gomock.Any())

	uc := New(d.tx, d.trips, d.history, d.clock, d.metrics, d.log)
	_, err := uc.Execute(context.Background(), id)
	if !errors.Is(err, domaintrip.ErrTripCompleted) {
		t.Fatalf("err = %v, want ErrTripCompleted", err)
	}
	d.trips.EXPECT().Finish(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
}

func TestExecute_RepoError(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	dbErr := errors.New("db down")
	id := uuid.New()
	tr := activeTrip(id)

	d.clock.EXPECT().Now().Return(now)
	passThrough(d.tx)
	d.trips.EXPECT().GetByID(gomock.Any(), id).Return(tr, nil)
	d.trips.EXPECT().Finish(gomock.Any(), id, now).Return(dbErr)
	d.log.EXPECT().Error("trip finish failed", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	uc := New(d.tx, d.trips, d.history, d.clock, d.metrics, d.log)
	_, err := uc.Execute(context.Background(), id)
	if !errors.Is(err, dbErr) {
		t.Fatalf("err = %v, want wrapped db error", err)
	}
}
