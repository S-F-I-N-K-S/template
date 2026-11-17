package health

import (
	"encoding/json"
	"net/http"

	api "github.com/S-F-I-N-K-S/template/internal/generated"
)

type Handler struct {
	pinger pinger
}

func New(p pinger) *Handler {
	return &Handler{pinger: p}
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	status := api.Ok
	code := http.StatusOK
	if err := h.pinger.Ping(r.Context()); err != nil {
		status = api.Unavailable
		code = http.StatusServiceUnavailable
	}
	writeResponse(w, code, api.HealthResponse{Status: status})
}

func writeResponse(w http.ResponseWriter, code int, body api.HealthResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
