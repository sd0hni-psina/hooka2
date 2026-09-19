package layout

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
	r.Private("POST /halls", h.createHall)
	r.Private("PATCH /halls/{hall_id}", h.updateHall)
	r.Private("DELETE /halls/{hall_id}", h.deleteHall)
	r.Private("POST /halls/{hall_id}/tables", h.createTable)
	r.Private("PATCH /tables/{table_id}", h.patchTable)
	r.Private("DELETE /tables/{table_id}", h.archiveTable)
	r.Private("PUT /halls/{hall_id}/layout", h.setLayout)
}

func claims(r *http.Request) (auth.Claims, error) {
	c, ok := auth.FromContext(r.Context())
	if !ok {
		return auth.Claims{}, domain.ErrUnauthorized
	}
	return c, nil
}

type hallRequest struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

func (h *Handler) createHall(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	var req hallRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	hall, err := h.svc.CreateHall(r.Context(), c, req.Name, req.SortOrder)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, hall)
	return nil
}

type hallPatch struct {
	Name      *string `json:"name"`
	SortOrder *int    `json:"sort_order"`
}

func (h *Handler) updateHall(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	var req hallPatch
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	hall, err := h.svc.UpdateHall(r.Context(), c, r.PathValue("hall_id"), HallInput(req))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, hall)
	return nil
}

func (h *Handler) deleteHall(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteHall(r.Context(), c, r.PathValue("hall_id")); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

type tableRequest struct {
	Label    string       `json:"label"`
	Shape    domain.Shape `json:"shape"`
	X        int          `json:"x"`
	Y        int          `json:"y"`
	W        int          `json:"w"`
	H        int          `json:"h"`
	Rotation int          `json:"rotation"`
	Seats    int          `json:"seats"`
}

func (h *Handler) createTable(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	var req tableRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	t, err := h.svc.CreateTable(r.Context(), c, r.PathValue("hall_id"), domain.TableFields(req))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, t)
	return nil
}

type tablePatch struct {
	Label    *string       `json:"label"`
	Shape    *domain.Shape `json:"shape"`
	X        *int          `json:"x"`
	Y        *int          `json:"y"`
	W        *int          `json:"w"`
	H        *int          `json:"h"`
	Rotation *int          `json:"rotation"`
	Seats    *int          `json:"seats"`
	Archived *bool         `json:"archived"`
}

func (h *Handler) patchTable(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	var req tablePatch
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	t, err := h.svc.PatchTable(r.Context(), c, r.PathValue("table_id"), domain.TablePatch(req))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, t)
	return nil
}

func (h *Handler) archiveTable(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	if err := h.svc.ArchiveTable(r.Context(), c, r.PathValue("table_id")); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

type placement struct {
	ID       string `json:"id"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	W        int    `json:"w"`
	H        int    `json:"h"`
	Rotation int    `json:"rotation"`
}

type layoutRequest struct {
	Tables []placement `json:"tables"`
}

func (h *Handler) setLayout(w http.ResponseWriter, r *http.Request) error {
	c, err := claims(r)
	if err != nil {
		return err
	}
	var req layoutRequest
	if err := httpx.Decode(r, &req); err != nil {
		return err
	}
	batch := make([]domain.Placement, len(req.Tables))
	for i, p := range req.Tables {
		batch[i] = domain.Placement(p)
	}
	tables, err := h.svc.SetLayout(r.Context(), c, r.PathValue("hall_id"), batch)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, tables)
	return nil
}
