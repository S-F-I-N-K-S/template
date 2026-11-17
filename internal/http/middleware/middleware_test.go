package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/http/domainerror"
	"github.com/S-F-I-N-K-S/template/internal/http/middleware"
)

const (
	levelInfo  = "info"
	levelError = "error"
)

type logEntry struct {
	level string
	msg   string
	args  []any
}

type fakeLogger struct {
	mu      sync.Mutex
	entries []logEntry
}

func (f *fakeLogger) InfoCtx(_ context.Context, msg string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, logEntry{level: levelInfo, msg: msg, args: args})
}

func (f *fakeLogger) ErrorCtx(_ context.Context, msg string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, logEntry{level: levelError, msg: msg, args: args})
}

func (f *fakeLogger) last() logEntry {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.entries[len(f.entries)-1]
}

func (f *fakeLogger) lastMsg() string {
	return f.last().msg
}

func (f *fakeLogger) entryWithMsg(t *testing.T, msg string) logEntry {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.entries {
		if e.msg == msg {
			return e
		}
	}
	t.Fatalf("no logged entry with msg %q", msg)
	return logEntry{}
}

func fieldValue(e logEntry, key string) (any, bool) {
	for i := 0; i+1 < len(e.args); i += 2 {
		if e.args[i] == key {
			return e.args[i+1], true
		}
	}
	return nil, false
}

type fakeInstruments struct {
	mu          sync.Mutex
	active      int64
	recorded    bool
	countedStat int
}

func (f *fakeInstruments) ActiveAdd(_ context.Context, delta int64, _, _ string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active += delta
}

func (f *fakeInstruments) RecordDuration(_ context.Context, _ float64, _, _ string, _ int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recorded = true
}

func (f *fakeInstruments) CountRequest(_ context.Context, _, _ string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.countedStat = status
}

func (f *fakeInstruments) snapshot() (active int64, recorded bool, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active, f.recorded, f.countedStat
}

func buildChain(l middleware.Logger, in middleware.Instruments, next http.Handler) http.Handler {
	h := middleware.NewMetrics(in)(next)
	h = middleware.NewLogger(l)(h)
	h = middleware.NewRecover(l)(h)
	return h
}

func TestMiddlewareChain_PanickingHandler(t *testing.T) {
	log := &fakeLogger{}
	in := &fakeInstruments{}

	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})

	h := buildChain(log, in, panicking)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("content-type = %q, want application/problem+json", ct)
	}
	var p api.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode problem: %v; body=%s", err, rec.Body.String())
	}
	if p.Code != domainerror.CodeInternal {
		t.Errorf("code = %q, want %q", p.Code, domainerror.CodeInternal)
	}

	active, recorded, status := in.snapshot()
	if active != 0 {
		t.Errorf("active_requests = %d, want 0 (counter must be decremented even on panic)", active)
	}
	if !recorded {
		t.Errorf("duration/count were not recorded for the panicking request")
	}
	if status != http.StatusInternalServerError {
		t.Errorf("counted status = %d, want 500", status)
	}

	reqEntry := log.entryWithMsg(t, "http request")
	if reqEntry.level != levelError {
		t.Errorf("log level = %q, want error for a 5xx response", reqEntry.level)
	}
	if v, ok := fieldValue(reqEntry, "status"); !ok || v != http.StatusInternalServerError {
		t.Errorf("logged status = %v, want 500", v)
	}
	if msg := log.lastMsg(); msg != "panic recovered" {
		t.Errorf("log message = %q, want %q", msg, "panic recovered")
	}
}

func TestMiddlewareChain_SuccessLogsAtInfo(t *testing.T) {
	log := &fakeLogger{}
	in := &fakeInstruments{}

	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := buildChain(log, in, ok)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	active, recorded, status := in.snapshot()
	if active != 0 {
		t.Errorf("active_requests = %d, want 0", active)
	}
	if !recorded || status != http.StatusOK {
		t.Errorf("recorded=%v status=%d, want recorded=true status=200", recorded, status)
	}
	if log.last().level != levelInfo {
		t.Errorf("log level = %q, want info for a 2xx response", log.last().level)
	}
}

func TestMiddlewareChain_ClientErrorLogsAtInfo(t *testing.T) {
	log := &fakeLogger{}
	in := &fakeInstruments{}

	badReq := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	h := buildChain(log, in, badReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if log.last().level != levelInfo {
		t.Errorf("log level = %q, want info for a 4xx response", log.last().level)
	}
}

func TestWriteInternalErrorProblem_NoLeakedPanicValue(t *testing.T) {
	log := &fakeLogger{}
	in := &fakeInstruments{}

	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(strings.Repeat("secret", 3))
	})

	h := buildChain(log, in, panicking)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), "secret") {
		t.Errorf("panic value leaked into response body: %s", rec.Body.String())
	}
}
