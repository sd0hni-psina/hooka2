package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"

	"hookah/server/internal/auth"
	"hookah/server/internal/domain"
	"hookah/server/internal/httpx"
)

const (
	venueA = "11111111-1111-4111-8111-111111111111"
	hall1  = "22222222-2222-4222-8222-222222222222"
	hall2  = "33333333-3333-4333-8333-333333333333"
)

type okChecker struct{}

func (okChecker) CheckSession(context.Context, auth.Claims) error { return nil }

// fakeSnapshots serves a tiny state and lets a test run code while the
// snapshot is being read.
type fakeSnapshots struct {
	mu     sync.Mutex
	during func()
}

func (f *fakeSnapshots) HallState(_ context.Context, venueID, hallID string, online []string, now time.Time) (any, error) {
	if hallID != hall1 && hallID != hall2 {
		return nil, domain.ErrNotFound
	}
	f.mu.Lock()
	during := f.during
	f.during = nil
	f.mu.Unlock()
	if during != nil {
		during()
	}
	return map[string]any{"hall": map[string]string{"id": hallID}, "online": online}, nil
}

type harness struct {
	hub    *Hub
	srv    *httptest.Server
	tokens *auth.Tokens
	snaps  *fakeSnapshots
}

func newHarness(t *testing.T) *harness {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := NewHub(log)
	tokens := auth.NewTokens("secret", time.Hour)
	guard := auth.NewGuard(tokens, okChecker{}, httpx.WriteError)
	snaps := &fakeSnapshots{}
	router := httpx.NewRouter(guard.Middleware)
	NewHandler(hub, guard, snaps, nil, log).Register(router)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return &harness{hub: hub, srv: srv, tokens: tokens, snaps: snaps}
}

func (h *harness) dial(t *testing.T, staffID string) *websocket.Conn {
	t.Helper()
	tok, _, err := h.tokens.Issue(auth.Claims{StaffID: staffID, VenueID: venueA, Role: domain.RoleMaster, SessionID: "s-" + staffID}, time.Now())
	require.NoError(t, err)
	url := "ws" + strings.TrimPrefix(h.srv.URL, "http") + "/api/v1/ws?access_token=" + tok
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.CloseNow() }) //nolint:errcheck // test cleanup
	return conn
}

func send(t *testing.T, c *websocket.Conn, typ string, data any) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"type": typ, "data": data})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, c.Write(ctx, websocket.MessageText, raw))
}

type received struct {
	Envelope
	Data json.RawMessage `json:"data"`
}

func recv(t *testing.T, c *websocket.Conn) received {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, raw, err := c.Read(ctx)
	require.NoError(t, err)
	var r received
	require.NoError(t, json.Unmarshal(raw, &r))
	return r
}

func subscribe(t *testing.T, c *websocket.Conn, hallID string) received {
	send(t, c, "subscribe", map[string]string{"hall_id": hallID})
	snap := recv(t, c)
	require.Equal(t, TypeSnapshot, snap.Type)
	return snap
}

func TestHub(t *testing.T) {
	t.Run("invalid token is rejected before upgrade", func(t *testing.T) {
		h := newHarness(t)
		res, err := http.Get(h.srv.URL + "/api/v1/ws?access_token=garbage")
		require.NoError(t, err)
		defer func() { require.NoError(t, res.Body.Close()) }()
		require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	})

	t.Run("ping echoes pong with server time and does not advance seq", func(t *testing.T) {
		h := newHarness(t)
		c := h.dial(t, "m1")
		h.hub.Publish(venueA, nil, TypePlanUpdated, map[string]string{})
		send(t, c, "ping", map[string]any{})
		pong := recv(t, c)
		require.Equal(t, TypePong, pong.Type)
		require.EqualValues(t, 1, pong.Seq)
		require.Nil(t, pong.HallID)
		var d PongData
		require.NoError(t, json.Unmarshal(pong.Data, &d))
		require.WithinDuration(t, time.Now(), d.ServerTime, 2*time.Second)
		require.EqualValues(t, 1, h.hub.Seq(venueA))
	})

	t.Run("subscribe replies with snapshot carrying the current seq", func(t *testing.T) {
		h := newHarness(t)
		h.hub.Publish(venueA, nil, TypePlanUpdated, map[string]string{})
		h.hub.Publish(venueA, nil, TypePlanUpdated, map[string]string{})
		c := h.dial(t, "m1")
		snap := subscribe(t, c, hall1)
		require.EqualValues(t, 2, snap.Seq)
		require.Equal(t, hall1, *snap.HallID)
		require.Contains(t, string(snap.Data), "m1", "snapshot lists the subscriber online")
	})

	t.Run("unknown hall replies with not_found error", func(t *testing.T) {
		h := newHarness(t)
		c := h.dial(t, "m1")
		send(t, c, "subscribe", map[string]string{"hall_id": "44444444-4444-4444-8444-444444444444"})
		msg := recv(t, c)
		require.Equal(t, TypeError, msg.Type)
		require.Contains(t, string(msg.Data), `"not_found"`)
	})

	t.Run("broadcasts reach every subscriber of the venue with gapless seq", func(t *testing.T) {
		h := newHarness(t)
		a := h.dial(t, "m1")
		b := h.dial(t, "m2")
		subscribe(t, a, hall1)
		subscribe(t, b, hall2)
		h1, h2 := hall1, hall2
		h.hub.Publish(venueA, &h1, TypeServiceDue, map[string]string{"n": "1"})
		h.hub.Publish(venueA, &h2, TypeServiceDue, map[string]string{"n": "2"})
		for _, c := range []*websocket.Conn{a, b} {
			first, second := recv(t, c), recv(t, c)
			require.EqualValues(t, 1, first.Seq)
			require.EqualValues(t, 2, second.Seq)
			require.Equal(t, hall1, *first.HallID)
			require.Equal(t, hall2, *second.HallID)
		}
	})

	t.Run("unsubscribed connection receives no broadcasts", func(t *testing.T) {
		h := newHarness(t)
		c := h.dial(t, "m1")
		h.hub.Publish(venueA, nil, TypePlanUpdated, map[string]string{})
		send(t, c, "ping", map[string]any{})
		require.Equal(t, TypePong, recv(t, c).Type)
	})

	t.Run("event published during snapshot read follows the snapshot", func(t *testing.T) {
		h := newHarness(t)
		c := h.dial(t, "m1")
		h.snaps.during = func() {
			h1 := hall1
			h.hub.Publish(venueA, &h1, TypeHookahStarted, map[string]string{})
		}
		snap := subscribe(t, c, hall1)
		require.EqualValues(t, 0, snap.Seq)
		next := recv(t, c)
		require.Equal(t, TypeHookahStarted, next.Type)
		require.EqualValues(t, 1, next.Seq)
	})

	t.Run("buffer overflow closes the connection with 1011", func(t *testing.T) {
		h := newHarness(t)
		c := h.dial(t, "m1")
		// Events published while the snapshot is read are held for the
		// connection; more than the buffer holds must drop it.
		h.snaps.during = func() {
			for i := 0; i <= sendBuffer; i++ {
				h.hub.Publish(venueA, nil, TypePlanUpdated, map[string]int{"i": i})
			}
		}
		send(t, c, "subscribe", map[string]string{"hall_id": hall1})
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var err error
		for err == nil {
			_, _, err = c.Read(ctx)
		}
		require.Equal(t, websocket.StatusInternalError, websocket.CloseStatus(err), fmt.Sprint(err))
	})

}
