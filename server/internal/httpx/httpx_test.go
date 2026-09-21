package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/domain"
)

type body struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

func write(t *testing.T, err error) (int, body) {
	t.Helper()
	rec := httptest.NewRecorder()
	WriteError(rec, httptest.NewRequest(http.MethodGet, "/", nil), err)
	var b body
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
	return rec.Code, b
}

func TestWriteErrorUsesClosedCodeList(t *testing.T) {
	cases := []struct {
		err    error
		code   string
		status int
	}{
		{domain.ErrValidation, "validation_failed", 400},
		{domain.ErrUnauthorized, "unauthorized", 401},
		{domain.ErrForbidden, "forbidden", 403},
		{domain.ErrNotFound, "not_found", 404},
		{domain.ErrInvalidJoinCode, "invalid_join_code", 404},
		{domain.ErrRateLimited, "rate_limited", 429},
		{domain.ErrTableOccupied, "table_occupied", 409},
		{domain.ErrTableBusy, "table_busy", 409},
		{domain.ErrHookahFinished, "hookah_finished", 409},
		{domain.ErrEventNotActionable, "event_not_actionable", 409},
		{domain.ErrPlanInvalid, "plan_invalid", 422},
		{errors.New("db exploded"), "internal", 500},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			status, b := write(t, fmt.Errorf("wrapped: %w", c.err))
			require.Equal(t, c.status, status)
			require.Equal(t, c.code, b.Error.Code)
			require.NotEmpty(t, b.Error.Message)
			require.NotContains(t, b.Error.Message, "exploded")
		})
	}
}

func TestWriteErrorDetails(t *testing.T) {
	t.Run("validation lists fields", func(t *testing.T) {
		v := &domain.ValidationError{}
		v.Add("email", domain.ReasonTaken)
		_, b := write(t, v)
		require.Equal(t, map[string]any{"email": "taken"}, b.Error.Details["fields"])
	})
	t.Run("rate limit carries retry_after_sec", func(t *testing.T) {
		_, b := write(t, &domain.RateLimitError{RetryAfterSec: 42})
		require.EqualValues(t, 42, b.Error.Details["retry_after_sec"])
	})
	t.Run("plan error carries reason", func(t *testing.T) {
		_, b := write(t, &domain.PlanError{Field: "grace_sec", Reason: "grace_out_of_range"})
		require.Equal(t, "grace_out_of_range", b.Error.Details["reason"])
	})
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	var v struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"a","extra":"b"}`))
	require.ErrorIs(t, Decode(req, &v), domain.ErrValidation)
}

func TestDecodeRejectsATruncatedBody(t *testing.T) {
	var v struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":`))
	err := Decode(req, &v)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, domain.ReasonInvalid, ve.Fields["body"])
}

func TestRouterRejectsABodyOverOneMiB(t *testing.T) {
	var v struct {
		Name string `json:"name"`
	}
	r := NewRouter(func(h http.Handler) http.Handler { return h })
	r.Public("POST /echo", func(w http.ResponseWriter, req *http.Request) error {
		if err := Decode(req, &v); err != nil {
			return err
		}
		NoContent(w)
		return nil
	})
	send := func(name string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, APIPrefix+"/echo", strings.NewReader(`{"name":"`+name+`"}`)))
		return rec
	}

	t.Run("a body just under the limit passes", func(t *testing.T) {
		require.Equal(t, http.StatusNoContent, send(strings.Repeat("a", maxBody-20)).Code)
	})
	t.Run("a body over the limit is too_long, not a silent cut", func(t *testing.T) {
		rec := send(strings.Repeat("a", maxBody))
		require.Equal(t, http.StatusBadRequest, rec.Code)
		var b body
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
		require.Equal(t, "validation_failed", b.Error.Code)
		require.Equal(t, map[string]any{"body": "too_long"}, b.Error.Details["fields"])
	})
}

func TestRouterUnknownAPIRouteIsJSONNotFound(t *testing.T) {
	r := NewRouter(func(h http.Handler) http.Handler { return h })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), `"not_found"`)
}

func TestRecoverTurnsPanicIntoInternal(t *testing.T) {
	h := Recover(discardLogger())(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), `"internal"`)
}
