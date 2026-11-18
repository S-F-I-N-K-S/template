package save_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domainposition "github.com/S-F-I-N-K-S/template/internal/domain/position"
	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	"github.com/S-F-I-N-K-S/template/internal/model"
	. "github.com/S-F-I-N-K-S/template/internal/usecase/trip/position/save"
)

var now = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

type deps struct {
	tx        *MocktxManager
	trips     *MocktripRepository
	positions *MockpositionRepository
	cancel    *Mockcanceller
	clock     *Mockclock
	metrics   *Mockmetrics
	log       *Mocklogger
}

func newDeps(t *testing.T) (*gomock.Controller, deps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	return ctrl, deps{
		tx:        NewMocktxManager(ctrl),
		trips:     NewMocktripRepository(ctrl),
		positions: NewMockpositionRepository(ctrl),
		cancel:    NewMockcanceller(ctrl),
		clock:     NewMockclock(ctrl),
		metrics:   NewMockmetrics(ctrl),
		log:       NewMocklogger(ctrl),
	}
}

func passThrough(tx *MocktxManager) {
	tx.EXPECT().Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		Times(1)
}

func validInput(tripID uuid.UUID) model.SavePositionInput {
	return model.SavePositionInput{
		TripID:      tripID,
		Coordinates: domaintrip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		RecordedAt:  now.Add(-time.Minute),
	}
}

func TestExecute_ValidationError(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	in := validInput(uuid.New())
	in.RecordedAt = now.Add(time.Hour) // далеко в будущем

	d.clock.EXPECT().Now().Return(now)
	d.log.EXPECT().InfoCtx(gomock.Any(), "position save rejected by validation", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	// tx.Do, репозитории и cancel не вызываются вовсе — ошибка до транзакции.

	uc := New(d.tx, d.trips, d.positions, d.cancel, d.clock, d.metrics, d.log)
	_, err := uc.Execute(context.Background(), in)

	var ve *domaintrip.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *domaintrip.ValidationError", err)
	}
}

func TestExecute_TripNotFound(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	tripID := uuid.New()
	in := validInput(tripID)

	d.clock.EXPECT().Now().Return(now)
	passThrough(d.tx)
	d.trips.EXPECT().AdvanceLastPositionAt(gomock.Any(), tripID, in.RecordedAt.UTC()).
		Return(domaintrip.ErrTripNotFound)
	d.log.EXPECT().InfoCtx(gomock.Any(), "position save: trip not found", gomock.Any(), gomock.Any())

	uc := New(d.tx, d.trips, d.positions, d.cancel, d.clock, d.metrics, d.log)
	_, err := uc.Execute(context.Background(), in)
	if !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("err = %v, want ErrTripNotFound", err)
	}
}

func TestExecute_TripCompleted(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	tripID := uuid.New()
	in := validInput(tripID)

	d.clock.EXPECT().Now().Return(now)
	passThrough(d.tx)
	d.trips.EXPECT().AdvanceLastPositionAt(gomock.Any(), tripID, in.RecordedAt.UTC()).
		Return(domaintrip.ErrTripCompleted)
	d.log.EXPECT().InfoCtx(gomock.Any(), "position save conflict: trip completed", gomock.Any(), gomock.Any())

	uc := New(d.tx, d.trips, d.positions, d.cancel, d.clock, d.metrics, d.log)
	_, err := uc.Execute(context.Background(), in)
	if !errors.Is(err, domaintrip.ErrTripCompleted) {
		t.Fatalf("err = %v, want ErrTripCompleted", err)
	}
}

func TestExecute_Success(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	tripID := uuid.New()
	in := validInput(tripID)

	d.clock.EXPECT().Now().Return(now)
	passThrough(d.tx)
	d.trips.EXPECT().AdvanceLastPositionAt(gomock.Any(), tripID, in.RecordedAt.UTC()).Return(nil)
	d.positions.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, p domainposition.Position) (domainposition.Position, error) {
			p.ID = 42
			return p, nil
		})
	d.cancel.EXPECT().Cancel(tripID)
	d.metrics.EXPECT().PositionSaved(gomock.Any())
	d.log.EXPECT().InfoCtx(gomock.Any(), "position saved", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	uc := New(d.tx, d.trips, d.positions, d.cancel, d.clock, d.metrics, d.log)
	got, err := uc.Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got.ID != 42 || got.TripID != tripID {
		t.Errorf("got = %+v", got)
	}
}

func TestExecute_RepoError(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	tripID := uuid.New()
	in := validInput(tripID)
	dbErr := errors.New("db down")

	d.clock.EXPECT().Now().Return(now)
	passThrough(d.tx)
	d.trips.EXPECT().AdvanceLastPositionAt(gomock.Any(), tripID, in.RecordedAt.UTC()).Return(dbErr)
	d.log.EXPECT().ErrorCtx(gomock.Any(), "position save failed", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	uc := New(d.tx, d.trips, d.positions, d.cancel, d.clock, d.metrics, d.log)
	_, err := uc.Execute(context.Background(), in)
	if !errors.Is(err, dbErr) {
		t.Fatalf("err = %v, want wrapped db error", err)
	}
}
