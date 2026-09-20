package hookah

import (
	"net/http"
	"time"

	"hookah/server/internal/auth"
	"hookah/server/internal/domain"
	"hookah/server/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(r *httpx.Router) {
	r.Private("GET /halls/{hall_id}/state", h.hallState)
	r.Private("POST /tables/{table_id}/hookah", h.start)
	r.Private("POST /hookahs/{hookah_id}/transfer", h.transfer)
	r.Private("POST /hookahs/{hookah_id}/finish", h.finish)
	r.Private("POST /service-events/{event_id}/claim", h.claim)
	r.Private("POST /service-events/{event_id}/confirm", h.confirm)
}

func claims(r *http.Request) (auth.Claims, error) {
	c, ok := auth.FromContext(r.Context())
	if !ok {
		return auth.Claims{}, domain.ErrUnauthorized
	}
	return c, nil
}

func (h *Handler) hallState(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	state, err := h.svc.HallState(r.Context(), c.VenueID, r.PathValue("hall_id"), nil, time.Now())
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, state)
	return nil
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	hookah, err := h.svc.Start(r.Context(), c, r.PathValue("table_id"), r.Header.Get("Idempotency-Key"))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, hookah)
	return nil
}

type transferRequest struct {
	ToTableID string `json:"to_table_id"`
}

func (h *Handler) transfer(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	var req transferRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	hookah, err := h.svc.Transfer(r.Context(), c, r.PathValue("hookah_id"), req.ToTableID)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, hookah)
	return nil
}

type finishRequest struct {
	Reason domain.FinishReason `json:"reason"`
}

func (h *Handler) finish(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	var req finishRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	hookah, err := h.svc.Finish(r.Context(), c, r.PathValue("hookah_id"), req.Reason)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, hookah)
	return nil
}

func (h *Handler) claim(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	event, err := h.svc.Claim(r.Context(), c, r.PathValue("event_id"))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, event)
	return nil
}

func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	event, err := h.svc.Confirm(r.Context(), c, r.PathValue("event_id"))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, event)
	return nil
}
