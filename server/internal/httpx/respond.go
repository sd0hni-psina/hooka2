package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"hookah/server/internal/domain"
)

const maxBody = 1 << 20

// JSON writes v with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// The status line is already out, so an encode error can only be logged.
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Debug("write json response", "err", err)
	}
}

func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// Decode reads a JSON body into v. Unknown fields and trailing data fail
// validation so clients learn about typos instead of silently losing input.
func Decode(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return bodyError(err)
	}
	if dec.More() {
		return bodyError(errors.New("trailing data"))
	}
	return nil
}

func bodyError(err error) error {
	v := &domain.ValidationError{}
	v.Add("body", domain.ReasonInvalid)
	return fmt.Errorf("decode body: %v: %w", err, v)
}
