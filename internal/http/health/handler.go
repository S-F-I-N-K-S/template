package health

import "net/http"

type Handler struct {
	pinger pinger
}

func New(p pinger) *Handler {
	return &Handler{pinger: p}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	panic("TODO: health.Handler.Health")
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	panic("TODO: health.Handler.Ready")
}
