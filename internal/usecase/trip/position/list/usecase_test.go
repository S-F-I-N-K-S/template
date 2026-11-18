package list_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domainposition "github.com/S-F-I-N-K-S/template/internal/domain/position"
	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	. "github.com/S-F-I-N-K-S/template/internal/usecase/trip/position/list"
)

type deps struct {
	trips     *MocktripReader
	positions *MockpositionReader
	log       *Mocklogger
}

func newDeps(t *testing.T) (*gomock.Controller, deps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	return ctrl, deps{
		trips:     NewMocktripReader(ctrl),
		positions: NewMockpositionReader(ctrl),
		log:       NewMocklogger(ctrl),
	}
}

func activeTrip(id uuid.UUID) domaintrip.Trip {
	return domaintrip.Trip{
		ID:         id,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: domaintrip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		EndPoint:   domaintrip.Coordinates{Latitude: 59.94, Longitude: 30.31},
		Price:      100,
		Status:     domaintrip.StatusActive,
		StartedAt:  time.Now().UTC(),
	}
}

func TestExecute_Success(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	tripID := uuid.New()
	want := []domainposition.Position{
		{ID: 1, TripID: tripID, Coordinates: domaintrip.Coordinates{Latitude: 1, Longitude: 2}, RecordedAt: time.Now().UTC()},
	}

	d.trips.EXPECT().GetByID(gomock.Any(), tripID).Return(activeTrip(tripID), nil)
	d.positions.EXPECT().ListByTrip(gomock.Any(), tripID).Return(want, nil)

	uc := New(d.trips, d.positions, d.log)
	got, err := uc.Execute(context.Background(), tripID)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Errorf("got = %+v", got)
	}
}

func TestExecute_EmptyRoute(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	tripID := uuid.New()
	d.trips.EXPECT().GetByID(gomock.Any(), tripID).Return(activeTrip(tripID), nil)
	d.positions.EXPECT().ListByTrip(gomock.Any(), tripID).Return(nil, nil)

	uc := New(d.trips, d.positions, d.log)
	got, err := uc.Execute(context.Background(), tripID)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got = %+v, want empty", got)
	}
}

func TestExecute_TripNotFound(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	tripID := uuid.New()
	d.trips.EXPECT().GetByID(gomock.Any(), tripID).Return(domaintrip.Trip{}, domaintrip.ErrTripNotFound)
	d.log.EXPECT().InfoCtx(gomock.Any(), "list positions: trip not found", gomock.Any(), gomock.Any())

	uc := New(d.trips, d.positions, d.log)
	_, err := uc.Execute(context.Background(), tripID)
	if !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("err = %v, want ErrTripNotFound", err)
	}
}

func TestExecute_RepoError(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	tripID := uuid.New()
	dbErr := errors.New("db down")
	d.trips.EXPECT().GetByID(gomock.Any(), tripID).Return(activeTrip(tripID), nil)
	d.positions.EXPECT().ListByTrip(gomock.Any(), tripID).Return(nil, dbErr)
	d.log.EXPECT().ErrorCtx(gomock.Any(), "list positions failed", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	uc := New(d.trips, d.positions, d.log)
	_, err := uc.Execute(context.Background(), tripID)
	if !errors.Is(err, dbErr) {
		t.Fatalf("err = %v, want wrapped db error", err)
	}
}
