package create_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	"github.com/S-F-I-N-K-S/template/internal/model"
	. "github.com/S-F-I-N-K-S/template/internal/usecase/trip/create"
)

type deps struct {
	tx      *MocktxManager
	trips   *MocktripRepository
	history *MockhistoryRepository
	clock   *Mockclock
	gen     *MockidGenerator
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
		gen:     NewMockidGenerator(ctrl),
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

func input() model.CreateTripInput {
	return model.CreateTripInput{
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: domaintrip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		EndPoint:   domaintrip.Coordinates{Latitude: 59.94, Longitude: 30.31},
		Price:      100,
	}
}

func TestExecute_Success(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	id := uuid.New()
	in := input()

	d.gen.EXPECT().New().Return(id)
	d.clock.EXPECT().Now().Return(now)
	passThrough(d.tx)
	d.trips.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	d.history.EXPECT().AppendHistory(gomock.Any(), id, "", string(domaintrip.StatusActive), "trip created").Return(nil)
	d.metrics.EXPECT().TripCreated(gomock.Any())
	d.log.EXPECT().Info("trip created", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	uc := New(d.tx, d.trips, d.history, d.clock, d.gen, d.metrics, d.log)
	tr, err := uc.Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if tr.ID != id || tr.Status != domaintrip.StatusActive {
		t.Errorf("trip = %+v", tr)
	}
}

func TestExecute_ValidationRejected(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	in := input()
	in.Price = -1

	d.gen.EXPECT().New().Return(uuid.New())
	d.clock.EXPECT().Now().Return(now)
	d.log.EXPECT().Info("trip create rejected by validation", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	uc := New(d.tx, d.trips, d.history, d.clock, d.gen, d.metrics, d.log)
	_, err := uc.Execute(context.Background(), in)
	var ve *domaintrip.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestExecute_DriverBusy(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	d.gen.EXPECT().New().Return(uuid.New())
	d.clock.EXPECT().Now().Return(now)
	passThrough(d.tx)
	d.trips.EXPECT().Save(gomock.Any(), gomock.Any()).Return(domaintrip.ErrDriverBusy)
	d.log.EXPECT().Info("trip create conflict: driver busy", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	uc := New(d.tx, d.trips, d.history, d.clock, d.gen, d.metrics, d.log)
	_, err := uc.Execute(context.Background(), input())
	if !errors.Is(err, domaintrip.ErrDriverBusy) {
		t.Fatalf("err = %v, want ErrDriverBusy", err)
	}
}

func TestExecute_RepoError(t *testing.T) {
	ctrl, d := newDeps(t)
	defer ctrl.Finish()

	dbErr := errors.New("db down")
	d.gen.EXPECT().New().Return(uuid.New())
	d.clock.EXPECT().Now().Return(now)
	passThrough(d.tx)
	d.trips.EXPECT().Save(gomock.Any(), gomock.Any()).Return(dbErr)
	d.log.EXPECT().Error("trip create failed", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	uc := New(d.tx, d.trips, d.history, d.clock, d.gen, d.metrics, d.log)
	_, err := uc.Execute(context.Background(), input())
	if !errors.Is(err, dbErr) {
		t.Fatalf("err = %v, want wrapped db error", err)
	}
}
