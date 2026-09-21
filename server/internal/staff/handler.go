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
	r.Public("POST /join", h.join)
	r.Public("POST /auth/refresh", h.refresh)
	r.Private("POST /auth/logout", h.logout)
	r.Private("DELETE /sessions/current", h.logout)
	r.Private("POST /sessions/push-token", h.pushToken)
	r.Private("GET /venues/current/staff", h.list)
	r.Private("PATCH /staff/{staff_id}", h.patch)
}

func claims(r *http.Request) (auth.Claims, error) {
	c, ok := auth.FromContext(r.Context())
	if !ok {
		return auth.Claims{}, domain.ErrUnauthorized
	}
	return c, nil
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	me, err := h.svc.Me(r.Context(), c)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, me)
	return nil
}

type joinRequest struct {
	Code        string          `json:"code"`
	DisplayName string          `json:"display_name"`
	DeviceID    string          `json:"device_id"`
	Platform    domain.Platform `json:"platform"`
}

func (h *Handler) join(w http.ResponseWriter, r *http.Request) error {
	var req joinRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	res, err := h.svc.Join(r.Context(), domain.JoinInput{
		Code: req.Code, DisplayName: req.DisplayName,
		Device: domain.Device{DeviceID: req.DeviceID, Platform: req.Platform},
	}, httpx.ClientIPFrom(r.Context()))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, res)
	return nil
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type tokensResponse struct {
	Tokens domain.Tokens `json:"tokens"`
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) error {
	var req refreshRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	tokens, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, tokensResponse{Tokens: tokens})
	return nil
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	if err := h.svc.Logout(r.Context(), c); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

type pushTokenRequest struct {
	ExpoPushToken string `json:"expo_push_token"`
}

func (h *Handler) pushToken(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	var req pushTokenRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	if err := h.svc.SetPushToken(r.Context(), c, req.ExpoPushToken); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	list, err := h.svc.List(r.Context(), c)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, list)
	return nil
}

type staffPatch struct {
	Role    *domain.Role `json:"role"`
	Blocked *bool        `json:"blocked"`
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	var req staffPatch
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	st, err := h.svc.Patch(r.Context(), c, r.PathValue("staff_id"), req.Role, req.Blocked)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, st)
	return nil
}
