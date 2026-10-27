package http

import (
	stdhttp "net/http"

	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/http/health"
	"github.com/S-F-I-N-K-S/template/internal/http/trip/create"
	"github.com/S-F-I-N-K-S/template/internal/http/trip/finish"
	"github.com/S-F-I-N-K-S/template/internal/http/trip/get"
)

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

type Deps struct{}

func NewRouter(d Deps) stdhttp.Handler {
	panic("TODO: http.NewRouter")
}
