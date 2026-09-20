package apitest

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

type Message struct {
	Type   string          `json:"type"`
	Seq    int64           `json:"seq"`
	HallID *string         `json:"hall_id"`
	Data   json.RawMessage `json:"data"`
}

type WS struct {
	env  *Env
	conn *websocket.Conn
}

// Subscribe opens a WS as token, subscribes to hallID and consumes the snapshot.
func (e *Env) Subscribe(token, hallID string) (*WS, Message) {
	e.T.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(e.Srv.URL, "http") + "/api/v1/ws?access_token=" + token
	conn, res, err := websocket.Dial(ctx, url, nil)
	require.NoError(e.T, err)
	if res != nil && res.Body != nil {
		require.NoError(e.T, res.Body.Close())
	}
	e.T.Cleanup(func() {
		if err := conn.CloseNow(); err != nil {
			e.T.Logf("close ws: %v", err)
		}
	})
	ws := &WS{env: e, conn: conn}
	raw, err := json.Marshal(map[string]any{"type": "subscribe", "data": map[string]string{"hall_id": hallID}})
	require.NoError(e.T, err)
	require.NoError(e.T, conn.Write(ctx, websocket.MessageText, raw))
	snap := ws.Next()
	require.Equal(e.T, "state.snapshot", snap.Type)
	return ws, snap
}

// Next reads one message or fails after two seconds.
func (w *WS) Next() Message {
	w.env.T.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, raw, err := w.conn.Read(ctx)
	require.NoError(w.env.T, err)
	var m Message
	require.NoError(w.env.T, json.Unmarshal(raw, &m))
	return m
}

// Until reads messages until one of type typ arrives.
func (w *WS) Until(typ string) Message {
	w.env.T.Helper()
	for {
		if m := w.Next(); m.Type == typ {
			return m
		}
	}
}

// Closed reports whether the server dropped this connection.
func (w *WS) Closed() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		if _, _, err := w.conn.Read(ctx); err != nil {
			return true
		}
	}
}
