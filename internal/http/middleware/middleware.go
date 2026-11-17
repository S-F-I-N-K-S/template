package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"

	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/http/domainerror"
)

type Logger interface {
	InfoCtx(ctx context.Context, msg string, args ...any)
	ErrorCtx(ctx context.Context, msg string, args ...any)
}

func NewRecover(l Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					l.ErrorCtx(r.Context(), "panic recovered",
						"path", r.URL.Path, "panic", p, "stack", string(debug.Stack()))
					writeInternalErrorProblem(w, r)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func writeInternalErrorProblem(w http.ResponseWriter, r *http.Request) {
	status := http.StatusInternalServerError
	instance := r.URL.Path
	body := api.Problem{
		Code:     domainerror.CodeInternal,
		Status:   int32(status),
		Title:    "Internal server error",
		Type:     "about:blank",
		Instance: &instance,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func RequestID(next http.Handler) http.Handler {
	return chimw.RequestID(next)
}

func NewLogger(l Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				status := ww.Status()
				if status == 0 {
					status = http.StatusInternalServerError
				}
				fields := []any{
					"method", r.Method,
					"route", RoutePattern(r),
					"status", status,
					"bytes", ww.BytesWritten(),
					"duration_ms", time.Since(start).Milliseconds(),
					"request_id", chimw.GetReqID(r.Context()),
				}
				if status >= http.StatusInternalServerError {
					l.ErrorCtx(r.Context(), "http request", fields...)
				} else {
					l.InfoCtx(r.Context(), "http request", fields...)
				}
			}()
			next.ServeHTTP(ww, r)
		})
	}
}

type Instruments interface {
	ActiveAdd(ctx context.Context, delta int64, method, route string)
	RecordDuration(ctx context.Context, seconds float64, method, route string, status int)
	CountRequest(ctx context.Context, method, route string, status int)
}

func NewMetrics(in Instruments) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			in.ActiveAdd(r.Context(), 1, r.Method, RoutePattern(r))
			defer func() {
				route := RoutePattern(r)
				in.ActiveAdd(r.Context(), -1, r.Method, route)

				status := ww.Status()
				if status == 0 {
					status = http.StatusInternalServerError
				}
				in.RecordDuration(r.Context(), time.Since(start).Seconds(), r.Method, route, status)
				in.CountRequest(r.Context(), r.Method, route, status)

				renameSpanByRoute(r, route)
			}()
			next.ServeHTTP(ww, r)
		})
	}
}

func renameSpanByRoute(r *http.Request, route string) {
	if route == "" {
		return
	}
	span := trace.SpanFromContext(r.Context())
	if span.IsRecording() {
		span.SetName(r.Method + " " + route)
	}
}

func RoutePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	return ""
}
