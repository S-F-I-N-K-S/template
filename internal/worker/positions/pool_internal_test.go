package positions

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type noopTicker struct{ ch chan time.Time }

func (t *noopTicker) C() <-chan time.Time { return t.ch }
func (t *noopTicker) Stop()               {}

type stubLister struct{}

func (stubLister) ListStaleActive(context.Context, time.Duration, time.Time) ([]StaleTrip, error) {
	return nil, nil
}

type stubPusher struct {
	mu       sync.Mutex
	calls    int
	err      error
	lastTrip uuid.UUID
}

func (p *stubPusher) RequestPosition(_ context.Context, _, tripID uuid.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	p.lastTrip = tripID
	return p.err
}

func (p *stubPusher) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

type stubMetrics struct {
	mu         sync.Mutex
	staleCheck []string
	queueSize  int64
}

func (m *stubMetrics) StaleCheck(_ context.Context, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.staleCheck = append(m.staleCheck, result)
}
func (m *stubMetrics) QueueSizeAdd(_ context.Context, delta int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queueSize += delta
}
func (*stubMetrics) WorkersActiveAdd(context.Context, int64)                {}
func (*stubMetrics) PositionRequestResult(context.Context, string, float64) {}

func (m *stubMetrics) results() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.staleCheck))
	copy(out, m.staleCheck)
	return out
}

type stubLogger struct{}

func (stubLogger) InfoCtx(context.Context, string, ...any)  {}
func (stubLogger) WarnCtx(context.Context, string, ...any)  {}
func (stubLogger) ErrorCtx(context.Context, string, ...any) {}

type stubClock struct{ now time.Time }

func (c stubClock) Now() time.Time { return c.now }

func newTestPool(push *stubPusher, m *stubMetrics) *Pool {
	return New(
		Config{Workers: 1, QueueSize: 1, RequestTimeout: time.Second},
		stubLister{}, push, m, stubLogger{}, stubClock{now: time.Now()},
		&noopTicker{ch: make(chan time.Time, 1)},
	)
}

func TestEnqueue_Dedup(t *testing.T) {
	m := &stubMetrics{}
	p := newTestPool(&stubPusher{}, m)
	tripID, driverID := uuid.New(), uuid.New()

	p.enqueue(context.Background(), StaleTrip{TripID: tripID, DriverID: driverID})

	p.enqueue(context.Background(), StaleTrip{TripID: tripID, DriverID: driverID})

	if got := m.results(); len(got) != 2 || got[0] != resultQueued || got[1] != resultDuplicate {
		t.Errorf("StaleCheck results = %v, want [queued duplicate]", got)
	}
	if len(p.queue) != 1 {
		t.Errorf("queue len = %d, want 1 (дубликат не должен был попасть в канал)", len(p.queue))
	}
}

func TestEnqueue_QueueFull_Dropped(t *testing.T) {
	m := &stubMetrics{}
	p := newTestPool(&stubPusher{}, m) // QueueSize: 1
	a, b := uuid.New(), uuid.New()

	p.enqueue(context.Background(), StaleTrip{TripID: a, DriverID: uuid.New()})
	p.enqueue(context.Background(), StaleTrip{TripID: b, DriverID: uuid.New()})

	if got := m.results(); len(got) != 2 || got[0] != resultQueued || got[1] != resultDropped {
		t.Errorf("StaleCheck results = %v, want [queued dropped]", got)
	}

	if _, stillMarked := p.queued[b]; stillMarked {
		t.Error("dropped trip should not stay marked as queued")
	}
}

func TestProcess_CancelledBeforeDequeue_SkipsPush(t *testing.T) {
	push := &stubPusher{}
	p := newTestPool(push, &stubMetrics{})
	tripID, driverID := uuid.New(), uuid.New()

	p.enqueue(context.Background(), StaleTrip{TripID: tripID, DriverID: driverID})
	p.Cancel(tripID)

	p.process(context.Background(), task{tripID: tripID, driverID: driverID})

	if push.callCount() != 0 {
		t.Errorf("RequestPosition calls = %d, want 0 (cancelled before processing)", push.callCount())
	}
}

func TestProcess_NotCancelled_CallsPush(t *testing.T) {
	push := &stubPusher{}
	p := newTestPool(push, &stubMetrics{})
	tripID, driverID := uuid.New(), uuid.New()

	p.enqueue(context.Background(), StaleTrip{TripID: tripID, DriverID: driverID})
	p.process(context.Background(), task{tripID: tripID, driverID: driverID})

	if push.callCount() != 1 || push.lastTrip != tripID {
		t.Errorf("RequestPosition calls = %d, lastTrip = %s, want 1 call for %s", push.callCount(), push.lastTrip, tripID)
	}
	if _, stillQueued := p.queued[tripID]; stillQueued {
		t.Error("trip should be unmarked after processing")
	}
}

func TestProcess_PushError_StillUnmarksTrip(t *testing.T) {
	push := &stubPusher{err: errors.New("push service unavailable")}
	p := newTestPool(push, &stubMetrics{})
	tripID, driverID := uuid.New(), uuid.New()

	p.enqueue(context.Background(), StaleTrip{TripID: tripID, DriverID: driverID})
	p.process(context.Background(), task{tripID: tripID, driverID: driverID})

	if push.callCount() != 1 {
		t.Errorf("RequestPosition calls = %d, want 1", push.callCount())
	}
	if _, stillQueued := p.queued[tripID]; stillQueued {
		t.Error("trip should be unmarked even after a failed push, so it can be retried on a later tick")
	}
}
