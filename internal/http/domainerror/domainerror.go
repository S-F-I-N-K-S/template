package domainerror

import (
	"log/slog"
	"net/http"

	api "github.com/S-F-I-N-K-S/template/internal/generated"
)

const (
	CodeInvalidRequest = "invalid_request"
	CodeTripNotFound   = "trip_not_found"
	CodeTripCompleted  = "trip_completed"
	CodeDriverBusy     = "driver_busy"
	CodeInternal       = "internal_error"
)

func Problem(err error) (status int, body api.Problem) {
	panic("TODO: domainerror.Problem")
}

type Writer struct{ log *slog.Logger }

func NewWriter(log *slog.Logger) *Writer {
	return &Writer{log: log}
}

func (w *Writer) Write(rw http.ResponseWriter, r *http.Request, err error) {
	panic("TODO: Writer.Write")
}

func (w *Writer) RequestError(rw http.ResponseWriter, r *http.Request, err error) {
	panic("TODO: Writer.RequestError")
}
