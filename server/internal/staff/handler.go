package staff

import (
	"net/http"

	"hookah/server/internal/auth"
	"hookah/server/internal/domain"
	"hookah/server/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(r *httpx.Router) {
	r.Private("GET /me", h.me)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) error {
	c, ok := auth.FromContext(r.Context())
	if !ok {
		return domain.ErrUnauthorized
	}
	me, err := h.svc.Me(r.Context(), c)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, me)
	return nil
}
