package venue

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
	r.Private("POST /venues/current/rotate-code", h.rotateCode)
}

type rotateCodeResponse struct {
	JoinCode string `json:"join_code"`
}

func (h *Handler) rotateCode(w http.ResponseWriter, r *http.Request) error {
	c, ok := auth.FromContext(r.Context())
	if !ok {
		return domain.ErrUnauthorized
	}
	code, err := h.svc.RotateCode(r.Context(), c)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, rotateCodeResponse{JoinCode: code})
	return nil
}
