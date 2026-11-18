package positions_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	. "github.com/S-F-I-N-K-S/template/internal/worker/positions"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

var now = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func TestPool_ScansAndPushesToAllStaleTrips(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lister := NewMocklister(ctrl)
	pusher := NewMockpusher(ctrl)
	metrics := NewMockmetrics(ctrl)
	log := NewMocklogger(ctrl)
	clock := NewMockclock(ctrl)
	ticker := newFakeTicker()

	tripA, driverA := uuid.New(), uuid.New()
	tripB, driverB := uuid.New(), uuid.New()

	var wg sync.WaitGroup
	wg.Add(2)

	lister.EXPECT().ListStaleActive(gomock.Any(), time.Minute, now).
		Return([]StaleTrip{{TripID: tripA, DriverID: driverA}, {TripID: tripB, DriverID: driverB}}, nil)
	metrics.EXPECT().StaleCheck(gomock.Any(), "queued").Times(2)
	metrics.EXPECT().QueueSizeAdd(gomock.Any(), int64(1)).Times(2)
	metrics.EXPECT().QueueSizeAdd(gomock.Any(), int64(-1)).Times(2)
	metrics.EXPECT().WorkersActiveAdd(gomock.Any(), gomock.Any()).AnyTimes()
	clock.EXPECT().Now().Return(now).AnyTimes()

	pusher.EXPECT().RequestPosition(gomock.Any(), driverA, tripA).DoAndReturn(
		func(context.Context, uuid.UUID, uuid.UUID) error { defer wg.Done(); return nil })
	pusher.EXPECT().RequestPosition(gomock.Any(), driverB, tripB).DoAndReturn(
		func(context.Context, uuid.UUID, uuid.UUID) error { defer wg.Done(); return nil })
	metrics.EXPECT().PositionRequestResult(gomock.Any(), "success", gomock.Any()).Times(2)
	log.EXPECT().InfoCtx(gomock.Any(), "position request sent", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(2)

	p := New(Config{Workers: 2, QueueSize: 10, StaleAfter: time.Minute, RequestTimeout: time.Second},
		lister, pusher, metrics, log, clock, ticker, nil)
	p.Start()

	ticker.Tick(now)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for both position requests")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() = %v, want nil", err)
	}
}

func TestPool_DuplicateTickIsNotReQueued(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lister := NewMocklister(ctrl)
	pusher := NewMockpusher(ctrl) // RequestPosition не ожидается вовсе
	metrics := NewMockmetrics(ctrl)
	log := NewMocklogger(ctrl)
	clock := NewMockclock(ctrl)
	ticker := newFakeTicker()

	tripID, driverID := uuid.New(), uuid.New()

	lister.EXPECT().ListStaleActive(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]StaleTrip{{TripID: tripID, DriverID: driverID}}, nil).Times(2)
	clock.EXPECT().Now().Return(now).AnyTimes()

	firstQueued := make(chan struct{})
	secondDuplicate := make(chan struct{})
	metrics.EXPECT().StaleCheck(gomock.Any(), "queued").DoAndReturn(
		func(context.Context, string) { close(firstQueued) })
	metrics.EXPECT().QueueSizeAdd(gomock.Any(), int64(1))
	metrics.EXPECT().StaleCheck(gomock.Any(), "duplicate").DoAndReturn(
		func(context.Context, string) { close(secondDuplicate) })

	p := New(Config{Workers: 0, QueueSize: 10, RequestTimeout: time.Second}, lister, pusher, metrics, log, clock, ticker, nil)
	p.Start()

	ticker.Tick(now)
	select {
	case <-firstQueued:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first tick to queue the trip")
	}

	ticker.Tick(now)
	select {
	case <-secondDuplicate:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the second tick to report a duplicate")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() = %v, want nil", err)
	}
}

func TestPool_PushErrorDoesNotStopWorker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lister := NewMocklister(ctrl)
	pusher := NewMockpusher(ctrl)
	metrics := NewMockmetrics(ctrl)
	log := NewMocklogger(ctrl)
	clock := NewMockclock(ctrl)
	ticker := newFakeTicker()

	failingTrip, failingDriver := uuid.New(), uuid.New()
	okTrip, okDriver := uuid.New(), uuid.New()

	lister.EXPECT().ListStaleActive(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]StaleTrip{{TripID: failingTrip, DriverID: failingDriver}, {TripID: okTrip, DriverID: okDriver}}, nil)
	metrics.EXPECT().StaleCheck(gomock.Any(), "queued").Times(2)
	metrics.EXPECT().QueueSizeAdd(gomock.Any(), gomock.Any()).AnyTimes()
	metrics.EXPECT().WorkersActiveAdd(gomock.Any(), gomock.Any()).AnyTimes()
	clock.EXPECT().Now().Return(now).AnyTimes()

	failed := make(chan struct{})
	succeeded := make(chan struct{})
	pushErr := errors.New("push service unavailable")
	pusher.EXPECT().RequestPosition(gomock.Any(), failingDriver, failingTrip).DoAndReturn(
		func(context.Context, uuid.UUID, uuid.UUID) error { defer close(failed); return pushErr })
	pusher.EXPECT().RequestPosition(gomock.Any(), okDriver, okTrip).DoAndReturn(
		func(context.Context, uuid.UUID, uuid.UUID) error { defer close(succeeded); return nil })
	metrics.EXPECT().PositionRequestResult(gomock.Any(), "error", gomock.Any())
	metrics.EXPECT().PositionRequestResult(gomock.Any(), "success", gomock.Any())
	log.EXPECT().WarnCtx(gomock.Any(), "push service request failed", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	log.EXPECT().InfoCtx(gomock.Any(), "position request sent", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	p := New(Config{Workers: 1, QueueSize: 10, RequestTimeout: time.Second}, lister, pusher, metrics, log, clock, ticker, nil)
	p.Start()
	ticker.Tick(now)

	for _, ch := range []chan struct{}{failed, succeeded} {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out: worker did not process both tasks after the first one errored")
		}
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() = %v, want nil", err)
	}
}

func TestPool_StopDrainsBufferedQueueWithinDeadline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lister := NewMocklister(ctrl)
	pusher := NewMockpusher(ctrl)
	metrics := NewMockmetrics(ctrl)
	log := NewMocklogger(ctrl)
	clock := NewMockclock(ctrl)
	ticker := newFakeTicker()

	tripID, driverID := uuid.New(), uuid.New()

	lister.EXPECT().ListStaleActive(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]StaleTrip{{TripID: tripID, DriverID: driverID}}, nil)
	metrics.EXPECT().StaleCheck(gomock.Any(), "queued")
	metrics.EXPECT().QueueSizeAdd(gomock.Any(), int64(1))
	metrics.EXPECT().QueueSizeAdd(gomock.Any(), int64(-1))
	metrics.EXPECT().WorkersActiveAdd(gomock.Any(), gomock.Any()).AnyTimes()
	clock.EXPECT().Now().Return(now).AnyTimes()

	processed := make(chan struct{})
	pusher.EXPECT().RequestPosition(gomock.Any(), driverID, tripID).DoAndReturn(
		func(context.Context, uuid.UUID, uuid.UUID) error { close(processed); return nil })
	metrics.EXPECT().PositionRequestResult(gomock.Any(), "success", gomock.Any())
	log.EXPECT().InfoCtx(gomock.Any(), "position request sent", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	p := New(Config{Workers: 1, QueueSize: 10, RequestTimeout: time.Second}, lister, pusher, metrics, log, clock, ticker, nil)
	p.Start()
	ticker.Tick(now)

	select {
	case <-processed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for buffered task to be processed")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() = %v, want nil", err)
	}
}

func TestPool_SameTripCanBeRequeuedOnNextTick(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lister := NewMocklister(ctrl)
	pusher := NewMockpusher(ctrl)
	metrics := NewMockmetrics(ctrl)
	log := NewMocklogger(ctrl)
	clock := NewMockclock(ctrl)
	ticker := newFakeTicker()

	tripID, driverID := uuid.New(), uuid.New()

	firstDone := make(chan struct{})
	secondDone := make(chan struct{})
	lister.EXPECT().ListStaleActive(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]StaleTrip{{TripID: tripID, DriverID: driverID}}, nil).Times(2)
	metrics.EXPECT().StaleCheck(gomock.Any(), "queued").Times(2)
	metrics.EXPECT().QueueSizeAdd(gomock.Any(), gomock.Any()).AnyTimes()
	metrics.EXPECT().WorkersActiveAdd(gomock.Any(), gomock.Any()).AnyTimes()
	metrics.EXPECT().PositionRequestResult(gomock.Any(), "success", gomock.Any()).Times(2)
	log.EXPECT().InfoCtx(gomock.Any(), "position request sent", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
	clock.EXPECT().Now().Return(now).AnyTimes()

	gomock.InOrder(
		pusher.EXPECT().RequestPosition(gomock.Any(), driverID, tripID).DoAndReturn(
			func(context.Context, uuid.UUID, uuid.UUID) error { close(firstDone); return nil }),
		pusher.EXPECT().RequestPosition(gomock.Any(), driverID, tripID).DoAndReturn(
			func(context.Context, uuid.UUID, uuid.UUID) error { close(secondDone); return nil }),
	)

	p := New(Config{Workers: 1, QueueSize: 10, RequestTimeout: time.Second}, lister, pusher, metrics, log, clock, ticker, nil)
	p.Start()

	ticker.Tick(now)
	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first position request")
	}

	ticker.Tick(now)
	select {
	case <-secondDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the second position request")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() = %v, want nil", err)
	}
}
