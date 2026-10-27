package domainerror

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	api "github.com/S-F-I-N-K-S/template/internal/generated"
)

type Logger interface {
	InfoCtx(ctx context.Context, msg string, args ...any)
	ErrorCtx(ctx context.Context, msg string, args ...any)
}

const (
	typeBlank = "about:blank"

	CodeInvalidRequest = "invalid_request"
	CodeTripNotFound   = "trip_not_found"
	CodeTripCompleted  = "trip_completed"
	CodeDriverBusy     = "driver_busy"
	CodeInternal       = "internal_error"
)

var problemByError = []struct {
	err    error
	status int
	code   string
	title  string
}{
	{domaintrip.ErrTripNotFound, http.StatusNotFound, CodeTripNotFound, "Trip not found"},
	{domaintrip.ErrTripCompleted, http.StatusConflict, CodeTripCompleted, "Trip already completed"},
	{domaintrip.ErrDriverBusy, http.StatusConflict, CodeDriverBusy, "Driver is busy"},
}

func Problem(err error) (status int, body api.Problem, ok bool) {
	for _, m := range problemByError {
		if errors.Is(err, m.err) {
			return m.status, api.Problem{
				Code:   m.code,
				Status: int32(m.status),
				Title:  m.title,
				Type:   typeBlank,
			}, true
		}
	}

	var ve *domaintrip.ValidationError
	if errors.As(err, &ve) {
		status = http.StatusBadRequest
		detail := ve.Field + ": " + ve.Reason
		return status, api.Problem{
			Code:   CodeInvalidRequest,
			Detail: &detail,
			Status: int32(status),
			Title:  "Invalid request",
			Type:   typeBlank,
		}, true
	}

	return 0, api.Problem{}, false
}

type Writer struct{ log Logger }

func NewWriter(log Logger) *Writer {
	return &Writer{log: log}
}

func (w *Writer) Write(rw http.ResponseWriter, r *http.Request, err error) {
	if status, body, ok := Problem(err); ok {
		w.writeProblem(rw, status, body, r.URL.Path)
		return
	}

	w.log.ErrorCtx(r.Context(), "unhandled domain error", "path", r.URL.Path, "err", err)
	status := http.StatusInternalServerError
	w.writeProblem(rw, status, api.Problem{
		Code:   CodeInternal,
		Status: int32(status),
		Title:  "Internal server error",
		Type:   typeBlank,
	}, r.URL.Path)
}

func (w *Writer) RequestError(rw http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusBadRequest
	w.log.InfoCtx(r.Context(), "request error", "path", r.URL.Path, "err", err)
	w.writeProblem(rw, status, api.Problem{
		Code:   CodeInvalidRequest,
		Status: int32(status),
		Title:  "Invalid request",
		Type:   typeBlank,
	}, r.URL.Path)
}

func (w *Writer) writeProblem(rw http.ResponseWriter, status int, body api.Problem, instance string) {
	body.Instance = &instance
	rw.Header().Set("Content-Type", "application/problem+json")
	rw.WriteHeader(status)
	if err := json.NewEncoder(rw).Encode(body); err != nil {
		w.log.ErrorCtx(context.Background(), "write problem response", "err", err)
	}
}
