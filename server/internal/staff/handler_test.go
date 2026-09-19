package staff_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/auth"
	"hookah/server/internal/domain"
	"hookah/server/internal/httpx"
	"hookah/server/internal/layout"
	"hookah/server/internal/staff"
	"hookah/server/internal/store/storetest"
	"hookah/server/internal/venue"
	"hookah/server/seed"
)

type env struct {
	srv    *httptest.Server
	tokens *auth.Tokens
	fx     seed.Fixture
	exec   func(sql string, args ...any)
}

func setup(t *testing.T) env {
	pool := storetest.Open(t)
	fx, err := seed.Seed(context.Background(), pool)
	require.NoError(t, err)

	svc := staff.NewService(pool, staff.NewRepository(), venue.NewRepository(), layout.NewRepository())
	tokens := auth.NewTokens("test-secret", 30*time.Minute)
	router := httpx.NewRouter(auth.NewGuard(tokens, svc, httpx.WriteError).Middleware)
	staff.NewHandler(svc).Register(router)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return env{srv: srv, tokens: tokens, fx: fx, exec: func(sql string, args ...any) {
		_, err := pool.Exec(context.Background(), sql, args...)
		require.NoError(t, err)
	}}
}

func (e env) token(t *testing.T, m seed.Member) string {
	tok, _, err := e.tokens.Issue(auth.Claims{StaffID: m.StaffID, VenueID: e.fx.VenueID, Role: m.Role, SessionID: m.SessionID}, time.Now())
	require.NoError(t, err)
	return tok
}

func (e env) get(t *testing.T, token string) (*http.Response, map[string]any) {
	req, err := http.NewRequest(http.MethodGet, e.srv.URL+"/api/v1/me", nil)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	var body map[string]any
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	return res, body
}

func errorCode(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func TestGetMe(t *testing.T) {
	e := setup(t)

	t.Run("master sees self, venue without code and both halls in order", func(t *testing.T) {
		res, body := e.get(t, e.token(t, e.fx.Masters[0]))
		require.Equal(t, http.StatusOK, res.StatusCode)

		var me staff.Me
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &me))
		require.Equal(t, e.fx.Masters[0].StaffID, me.Staff.ID)
		require.Equal(t, "Ерлан", me.Staff.DisplayName)
		require.Equal(t, domain.RoleMaster, me.Staff.Role)
		require.False(t, me.Staff.Blocked)
		require.Equal(t, e.fx.VenueID, me.Venue.ID)
		require.Nil(t, me.Venue.JoinCode)
		require.Len(t, me.Halls, 2)
		require.Equal(t, "Основной зал", me.Halls[0].Name)
		require.Equal(t, "Второй зал", me.Halls[1].Name)
	})

	t.Run("owner sees the join code", func(t *testing.T) {
		res, body := e.get(t, e.token(t, e.fx.Owner))
		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, seed.JoinCode, body["venue"].(map[string]any)["join_code"])
	})

	t.Run("missing token is unauthorized", func(t *testing.T) {
		res, body := e.get(t, "")
		require.Equal(t, http.StatusUnauthorized, res.StatusCode)
		require.Equal(t, "unauthorized", errorCode(body))
	})

	t.Run("revoked session is unauthorized", func(t *testing.T) {
		m := e.fx.Masters[1]
		e.exec(`update staff_sessions set revoked_at = now() where id = $1`, m.SessionID)
		res, body := e.get(t, e.token(t, m))
		require.Equal(t, http.StatusUnauthorized, res.StatusCode)
		require.Equal(t, "unauthorized", errorCode(body))
	})

	t.Run("blocked master is forbidden", func(t *testing.T) {
		m := e.fx.Masters[2]
		e.exec(`update staff set blocked_at = now() where id = $1`, m.StaffID)
		res, body := e.get(t, e.token(t, m))
		require.Equal(t, http.StatusForbidden, res.StatusCode)
		require.Equal(t, "forbidden", errorCode(body))
	})
}
