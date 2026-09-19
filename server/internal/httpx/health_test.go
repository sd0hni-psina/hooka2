package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type pingFunc func(context.Context) error

func (f pingFunc) Ping(ctx context.Context) error { return f(ctx) }

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	Health(pingFunc(func(context.Context) error { return nil })).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())

	rec = httptest.NewRecorder()
	Health(pingFunc(func(context.Context) error { return errors.New("down") })).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.JSONEq(t, `{"status":"unavailable"}`, rec.Body.String())
}
