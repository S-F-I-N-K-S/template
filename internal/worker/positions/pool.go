package positions

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	resultDuplicate = "duplicate"
	resultQueued    = "queued"
	resultDropped   = "dropped"
	resultSuccess   = "success"
	resultError     = "error"
)

type Config struct {
	CheckInterval  time.Duration
	StaleAfter     time.Duration
	Workers        int
	QueueSize      int
	RequestTimeout time.Duration
}

type task struct {
	tripID   uuid.UUID
	driverID uuid.UUID
}

type Pool struct {
	cfg    Config
	scan   lister
	push   pusher
	m      metrics
	log    logger
	clk    clock
	ticker Ticker

	queue chan task

	mu     sync.Mutex
	queued map[uuid.UUID]struct{} // поездки сейчас в очереди или в обработке — дедупликация и отмена

	wg       sync.WaitGroup
	stopScan chan struct{}
	scanDone chan struct{}

	cancel context.CancelFunc
}

func New(cfg Config, scan lister, push pusher, m metrics, log logger, clk clock, ticker Ticker) *Pool {
	return &Pool{
		cfg:      cfg,
		scan:     scan,
		push:     push,
		m:        m,
		log:      log,
		clk:      clk,
		ticker:   ticker,
		queue:    make(chan task, cfg.QueueSize),
		queued:   make(map[uuid.UUID]struct{}, cfg.QueueSize),
		stopScan: make(chan struct{}),
		scanDone: make(chan struct{}),
	}
}

func (p *Pool) Start() {
	rootCtx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	for range p.cfg.Workers {
		p.wg.Add(1)
		go p.runWorker(rootCtx)
	}
	go p.runScanner(rootCtx)
}

func (p *Pool) Stop(ctx context.Context) error {
	close(p.stopScan)
	<-p.scanDone
	p.ticker.Stop()

	close(p.queue)

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	var err error
	select {
	case <-done:
	case <-ctx.Done():
		err = ctx.Err()
	}

	if p.cancel != nil {
		p.cancel()
	}
	return err
}

func (p *Pool) Cancel(tripID uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.queued, tripID)
}

func (p *Pool) runScanner(ctx context.Context) {
	defer close(p.scanDone)
	for {
		select {
		case <-p.stopScan:
			return
		case <-ctx.Done():
			return
		case <-p.ticker.C():
			p.tick(ctx)
		}
	}
}

func (p *Pool) tick(ctx context.Context) {
	trips, err := p.scan.ListStaleActive(ctx, p.cfg.StaleAfter, p.clk.Now())
	if err != nil {
		p.log.ErrorCtx(ctx, "stale trips scan failed", "err", err)
		return
	}
	for _, t := range trips {
		p.enqueue(ctx, t)
	}
}

func (p *Pool) enqueue(ctx context.Context, t StaleTrip) {
	p.mu.Lock()
	if _, ok := p.queued[t.TripID]; ok {
		p.mu.Unlock()
		p.m.StaleCheck(ctx, resultDuplicate)
		return
	}

	p.queued[t.TripID] = struct{}{}

	select {
	case p.queue <- task{tripID: t.TripID, driverID: t.DriverID}:
		p.mu.Unlock()
		p.m.StaleCheck(ctx, resultQueued)
		p.m.QueueSizeAdd(ctx, 1)
	default:
		delete(p.queued, t.TripID)
		p.mu.Unlock()
		p.m.StaleCheck(ctx, resultDropped)
		p.log.WarnCtx(ctx, "position request queue is full, dropping stale check",
			"trip_id", t.TripID, "driver_id", t.DriverID)
	}
}

func (p *Pool) runWorker(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case t, ok := <-p.queue:
			if !ok {
				return
			}
			p.m.QueueSizeAdd(ctx, -1)
			p.process(ctx, t)
		}
	}
}

func (p *Pool) process(ctx context.Context, t task) {
	p.mu.Lock()
	_, stillQueued := p.queued[t.tripID]
	p.mu.Unlock()
	if !stillQueued {
		return
	}

	p.m.WorkersActiveAdd(ctx, 1)
	defer p.m.WorkersActiveAdd(ctx, -1)

	reqCtx, cancel := context.WithTimeout(ctx, p.cfg.RequestTimeout)
	defer cancel()

	start := p.clk.Now()
	pushErr := p.push.RequestPosition(reqCtx, t.driverID, t.tripID)
	elapsed := p.clk.Now().Sub(start).Seconds()

	p.mu.Lock()
	delete(p.queued, t.tripID)
	p.mu.Unlock()

	if pushErr != nil {
		p.m.PositionRequestResult(ctx, resultError, elapsed)
		p.log.WarnCtx(ctx, "push service request failed",
			"trip_id", t.tripID, "driver_id", t.driverID, "err", pushErr)
		return
	}
	p.m.PositionRequestResult(ctx, resultSuccess, elapsed)
	p.log.InfoCtx(ctx, "position request sent", "trip_id", t.tripID, "driver_id", t.driverID)
}
