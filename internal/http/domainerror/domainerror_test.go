package domainerror_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/http/domainerror"
)

type discardLogger struct{}

func (discardLogger) InfoCtx(context.Context, string, ...any)  {}
func (discardLogger) ErrorCtx(context.Context, string, ...any) {}

func newWriter() *domainerror.Writer {
	return domainerror.NewWriter(discardLogger{})
}

func TestProblem_TableDriven(t *testing.T) {
	internalErr := errors.New("pg: connection reset by peer")
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantOK     bool
	}{
		{"not found", domaintrip.ErrTripNotFound, http.StatusNotFound, domainerror.CodeTripNotFound, true},
		{"not found wrapped", fmt.Errorf("get: %w", domaintrip.ErrTripNotFound), http.StatusNotFound, domainerror.CodeTripNotFound, true},
		{"completed", domaintrip.ErrTripCompleted, http.StatusConflict, domainerror.CodeTripCompleted, true},
		{"driver busy", domaintrip.ErrDriverBusy, http.StatusConflict, domainerror.CodeDriverBusy, true},
		{"validation", &domaintrip.ValidationError{Field: "price", Reason: "must be >= 0"}, http.StatusBadRequest, domainerror.CodeInvalidRequest, true},
		{"internal", internalErr, 0, "", false},
		{"nil", nil, 0, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, body, ok := domainerror.Problem(tc.err)
			if ok != tc.wantOK || status != tc.wantStatus {
				t.Fatalf("Problem = (%d, %v), want (%d, %v)", status, ok, tc.wantStatus, tc.wantOK)
			}
			if ok && body.Code != tc.wantCode {
				t.Errorf("code = %q, want %q", body.Code, tc.wantCode)
			}
		})
	}
}

func TestWrite_NoInternalDetailsLeak(t *testing.T) {
	w := newWriter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/1", nil)

	secret := errors.New("pq: password secret@host failed")
	w.Write(rec, req, fmt.Errorf("repo: %w", secret))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body api.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body.String())
	}
	if body.Code != domainerror.CodeInternal {
		t.Errorf("code = %q, want internal_error", body.Code)
	}
	if strings.Contains(rec.Body.String(), "password") || (body.Detail != nil && strings.Contains(*body.Detail, "pq")) {
		t.Fatalf("internal error details leaked: %s", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("content-type = %q", ct)
	}
}

func TestWrite_ValidationDetail(t *testing.T) {
	w := newWriter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", nil)

	w.Write(rec, req, fmt.Errorf("create: %w", &domaintrip.ValidationError{Field: "price", Reason: "must be >= 0"}))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body api.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != domainerror.CodeInvalidRequest {
		t.Errorf("code = %q, want invalid_request", body.Code)
	}
	if body.Detail == nil || !strings.Contains(*body.Detail, "price") {
		t.Errorf("detail = %v, want field mentioned", body.Detail)
	}
}

func TestRequestError(t *testing.T) {
	w := newWriter()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", nil)

	w.RequestError(rec, req, errors.New("json: cannot unmarshal"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body api.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != domainerror.CodeInvalidRequest {
		t.Errorf("code = %q, want invalid_request", body.Code)
	}
}
