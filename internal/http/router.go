package http

import (
	stdhttp "net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/http/domainerror"
	"github.com/S-F-I-N-K-S/template/internal/http/health"
	"github.com/S-F-I-N-K-S/template/internal/http/middleware"
	"github.com/S-F-I-N-K-S/template/internal/http/trip/create"
	"github.com/S-F-I-N-K-S/template/internal/http/trip/finish"
	"github.com/S-F-I-N-K-S/template/internal/http/trip/get"
)

type Deps struct {
	Create  *create.Handler
	Get     *get.Handler
	Finish  *finish.Handler
	Health  *health.Handler
	Errs    *domainerror.Writer
	Logger  middleware.Logger
	Metrics middleware.Instruments
	Tracer  trace.TracerProvider
}

type Server struct {
	api.Unimplemented
	create *create.Handler
	get    *get.Handler
	finish *finish.Handler
	health *health.Handler
}

func (s *Server) CreateTrip(w stdhttp.ResponseWriter, r *stdhttp.Request, params api.CreateTripParams) {
	s.create.Handle(w, r, params)
}

func (s *Server) GetTrip(w stdhttp.ResponseWriter, r *stdhttp.Request, tripID api.TripId) {
	s.get.Handle(w, r, tripID)
}

func (s *Server) FinishTrip(w stdhttp.ResponseWriter, r *stdhttp.Request, tripID api.TripId) {
	s.finish.Handle(w, r, tripID)
}

func (s *Server) Health(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	s.health.Health(w, r)
}

func (s *Server) Ready(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	s.health.Ready(w, r)
}

func NewRouter(d Deps) stdhttp.Handler {
	r := chi.NewRouter()

	r.Use(middleware.NewRecover(d.Logger))
	r.Use(middleware.RequestID)
	r.Use(middleware.NewMetrics(d.Metrics))
	r.Use(middleware.NewLogger(d.Logger))

	handler := api.HandlerWithOptions(&Server{
		create: d.Create,
		get:    d.Get,
		finish: d.Finish,
		health: d.Health,
	}, api.ChiServerOptions{
		BaseRouter:       r,
		ErrorHandlerFunc: d.Errs.RequestError,
	})

	return otelhttp.NewHandler(
		handler,
		"",
		otelhttp.WithTracerProvider(d.Tracer),
		otelhttp.WithSpanNameFormatter(func(_ string, req *stdhttp.Request) string {
			return req.Method + " " + req.URL.Path
		}),
	)
}
