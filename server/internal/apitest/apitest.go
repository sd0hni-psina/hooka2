// Package apitest runs the real router against a per-test database, for API
// tests that go through HTTP like the app does.
package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"hookah/server/internal/account"
	"hookah/server/internal/auth"
	"hookah/server/internal/domain"
	"hookah/server/internal/hookah"
	"hookah/server/internal/httpx"
	"hookah/server/internal/layout"
	"hookah/server/internal/realtime"
	"hookah/server/internal/staff"
	"hookah/server/internal/store/storetest"
	"hookah/server/internal/venue"
	"hookah/server/seed"
)

const (
	Secret     = "test-secret"
	AccessTTL  = 30 * time.Minute
	RefreshTTL = 90 * 24 * time.Hour
)

type Env struct {
	T      *testing.T
	Pool   *pgxpool.Pool
	Srv    *httptest.Server
	Tokens *auth.Tokens
	Hub    *realtime.Hub
	Hookah *hookah.Service
	Fx     seed.Fixture
}

// New builds the full API on a fresh database. withSeed loads the fixture.
// The server trusts X-Forwarded-For like the prod deploy behind Caddy, so a
// test picks its client address with that header.
func New(t *testing.T, withSeed bool) *Env {
	t.Helper()
	pool := storetest.Open(t)
	var fx seed.Fixture
	if withSeed {
		var err error
		fx, err = seed.Seed(context.Background(), pool)
		require.NoError(t, err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	hub := realtime.NewHub(log)
	venueRepo, layoutRepo, staffRepo := venue.NewRepository(), layout.NewRepository(), staff.NewRepository()
	tokens := auth.NewTokens(Secret, AccessTTL)
	staffSvc := staff.NewService(pool, staffRepo, venueRepo, layoutRepo, tokens, RefreshTTL, hub)
	venueSvc := venue.NewService(pool, venueRepo, hub)
	accountSvc := account.NewService(pool, account.NewRepository(), staffRepo, staffSvc, venueRepo, venueSvc)
	hookahSvc := hookah.NewService(pool, hookah.NewRepository(), layoutRepo, venueRepo, staffRepo, hub)
	layoutSvc := layout.NewService(pool, layoutRepo, hub)

	guard := auth.NewGuard(tokens, staffSvc, httpx.WriteError)
	router := httpx.NewRouter(guard.Middleware)
	staff.NewHandler(staffSvc).Register(router)
	account.NewHandler(accountSvc).Register(router)
	venue.NewHandler(venueSvc).Register(router)
	layout.NewHandler(layoutSvc).Register(router)
	hookah.NewHandler(hookahSvc).Register(router)
	realtime.NewHandler(hub, guard, hookah.Snapshots{Svc: hookahSvc}, nil, log).Register(router)

	srv := httptest.NewServer(httpx.Chain(router, httpx.RequestID, httpx.Recover(log), httpx.ClientIP(true)))
	t.Cleanup(srv.Close)
	return &Env{T: t, Pool: pool, Srv: srv, Tokens: tokens, Hub: hub, Hookah: hookahSvc, Fx: fx}
}

// TokenFor signs an access token for a seeded member.
func (e *Env) TokenFor(m seed.Member) string {
	tok, _, err := e.Tokens.Issue(auth.Claims{StaffID: m.StaffID, VenueID: e.Fx.VenueID, Role: m.Role, SessionID: m.SessionID}, time.Now())
	require.NoError(e.T, err)
	return tok
}

type Response struct {
	Status int
	Body   map[string]any
	Raw    []byte
}

// Code returns error.code of an error body.
func (r Response) Code() string {
	e, _ := r.Body["error"].(map[string]any)
	c, _ := e["code"].(string)
	return c
}

func (r Response) Details() map[string]any {
	e, _ := r.Body["error"].(map[string]any)
	d, _ := e["details"].(map[string]any)
	return d
}

// Do sends a JSON request. body may be nil.
func (e *Env) Do(method, path, token string, body any, headers ...string) Response {
	e.T.Helper()
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(e.T, err)
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, e.Srv.URL+httpx.APIPrefix+path, rdr)
	require.NoError(e.T, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	require.NoError(e.T, err)
	defer func() { require.NoError(e.T, res.Body.Close()) }()
	raw, err := io.ReadAll(res.Body)
	require.NoError(e.T, err)
	out := Response{Status: res.StatusCode, Raw: raw}
	// Array responses stay in Raw; read them with Decode.
	if len(raw) > 0 && raw[0] == '{' {
		require.NoError(e.T, json.Unmarshal(raw, &out.Body), string(raw))
	}
	return out
}

// Decode re-reads the raw body into v.
func (r Response) Decode(t *testing.T, v any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(r.Raw, v))
}

// Exec runs SQL for test setup.
func (e *Env) Exec(sql string, args ...any) {
	e.T.Helper()
	_, err := e.Pool.Exec(context.Background(), sql, args...)
	require.NoError(e.T, err)
}

// Count returns select count(*) for a query tail, e.g. "staff where venue_id = $1".
func (e *Env) Count(tail string, args ...any) int {
	e.T.Helper()
	var n int
	require.NoError(e.T, e.Pool.QueryRow(context.Background(), "select count(*) from "+tail, args...).Scan(&n))
	return n
}

// Device returns a join or login device payload.
func Device(id string) map[string]any {
	return map[string]any{"device_id": id, "platform": string(domain.PlatformAndroid)}
}

// With merges maps into a request body.
func With(parts ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, p := range parts {
		for k, v := range p {
			out[k] = v
		}
	}
	return out
}
