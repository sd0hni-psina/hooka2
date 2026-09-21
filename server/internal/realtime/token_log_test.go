package realtime_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
)

// The WS token travels in the query string because React Native cannot set
// headers on a socket. Nothing may write that string down, tech.md section 19.16.
func TestAccessTokenNeverReachesTheLog(t *testing.T) {
	e := apitest.New(t, true)
	token := e.TokenFor(e.Fx.Masters[0])

	ws, _ := e.Subscribe(token, e.Fx.HallIDs[0])
	e.Hub.CloseSession(e.Fx.VenueID, e.Fx.Masters[0].SessionID)
	require.Equal(t, websocket.StatusPolicyViolation, ws.CloseStatus())

	// The refusal paths log too: a broken token, a revoked session, a blocked master.
	dial := func(query string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, res, _ := websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.Srv.URL, "http")+"/api/v1/ws?"+query, nil)
		if res != nil && res.Body != nil {
			require.NoError(t, res.Body.Close())
		}
	}
	revoked := e.TokenFor(e.Fx.Masters[1])
	e.Exec(`update staff_sessions set revoked_at = now() where id = $1`, e.Fx.Masters[1].SessionID)
	blocked := e.TokenFor(e.Fx.Masters[2])
	e.Exec(`update staff set blocked_at = now() where id = $1`, e.Fx.Masters[2].StaffID)
	dial("access_token=" + revoked)
	dial("access_token=" + blocked)
	dial("access_token=not-a-jwt-SECRETMARKER")

	logs := e.LogText()
	require.Contains(t, logs, "/api/v1/ws", "the requests were logged, so the check below means something")
	for _, secret := range []string{token, revoked, blocked, "SECRETMARKER", "access_token"} {
		require.NotContains(t, logs, secret)
	}
}
