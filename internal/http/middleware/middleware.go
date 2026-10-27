package middleware

import (
	"context"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
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
					w.WriteHeader(http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func RequestID(next http.Handler) http.Handler {
	return chimw.RequestID(next)
}

func NewLogger(l Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			l.InfoCtx(r.Context(), "http request",
				"method", r.Method,
				"route", RoutePattern(r),
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", chimw.GetReqID(r.Context()),
			)
		})
	}
}

type Instruments interface {
	ActiveAdd(ctx context.Context, delta int64)
	RecordDuration(ctx context.Context, seconds float64, method, route string, status int)
	CountRequest(ctx context.Context, method, route string, status int)
}

func NewMetrics(in Instruments) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			in.ActiveAdd(r.Context(), 1)
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			in.ActiveAdd(r.Context(), -1)

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			route := RoutePattern(r)
			in.RecordDuration(r.Context(), time.Since(start).Seconds(), r.Method, route, status)
			in.CountRequest(r.Context(), r.Method, route, status)
		})
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
