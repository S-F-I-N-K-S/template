package get

import (
	"net/http"

	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/http/domainerror"
	"github.com/S-F-I-N-K-S/template/internal/http/trip/create"
)

type Handler struct {
	uc   usecase
	errs *domainerror.Writer
}

func New(uc usecase, errs *domainerror.Writer) *Handler {
	return &Handler{uc: uc, errs: errs}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	t, err := h.uc.Execute(r.Context(), tripID)
	if err != nil {
		h.errs.Write(w, r, err)
		return
	}

	create.WriteTrip(w, http.StatusOK, t)
}
