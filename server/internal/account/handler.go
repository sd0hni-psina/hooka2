package account

import (
	"net/http"

	"hookah/server/internal/domain"
	"hookah/server/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(r *httpx.Router) {
	r.Public("POST /auth/register", h.register)
	r.Public("POST /auth/login", h.login)
}

type registerRequest struct {
	Email       string          `json:"email"`
	Password    string          `json:"password"`
	DisplayName string          `json:"display_name"`
	VenueName   string          `json:"venue_name"`
	DeviceID    string          `json:"device_id"`
	Platform    domain.Platform `json:"platform"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) error {
	var req registerRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	res, err := h.svc.Register(r.Context(), domain.RegisterInput{
		Email: req.Email, Password: req.Password, DisplayName: req.DisplayName, VenueName: req.VenueName,
		Device: domain.Device{DeviceID: req.DeviceID, Platform: req.Platform},
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, res)
	return nil
}

type loginRequest struct {
	Email    string          `json:"email"`
	Password string          `json:"password"`
	DeviceID string          `json:"device_id"`
	Platform domain.Platform `json:"platform"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) error {
	var req loginRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	res, err := h.svc.Login(r.Context(), domain.LoginInput{
		Email: req.Email, Password: req.Password,
		Device: domain.Device{DeviceID: req.DeviceID, Platform: req.Platform},
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, res)
	return nil
}
