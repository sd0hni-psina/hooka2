package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"hookah/server/internal/auth"
	"hookah/server/internal/httpx"
)

const (
	// Clients ping every 20 s; silence for 60 s drops the connection.
	readTimeout  = 60 * time.Second
	writeTimeout = 10 * time.Second
)

type Handler struct {
	hub       *Hub
	guard     *auth.Guard
	snapshots Snapshotter
	origins   []string
	log       *slog.Logger
}

func NewHandler(hub *Hub, guard *auth.Guard, snapshots Snapshotter, origins []string, log *slog.Logger) *Handler {
	return &Handler{hub: hub, guard: guard, snapshots: snapshots, origins: origins, log: log}
}

func (h *Handler) Register(r *httpx.Router) {
	// Auth runs inside: the token travels in the query because RN cannot set WS headers.
	r.Public("GET /ws", h.serve)
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request) error {
	claims, err := h.guard.Authenticate(r.Context(), r.URL.Query().Get("access_token"))
	if err != nil {
		return err
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.origins})
	if err != nil {
		// Accept already wrote the HTTP error.
		h.log.DebugContext(r.Context(), "ws accept", "err", err)
		return nil
	}
	c := newClient(claims.VenueID, claims.StaffID, claims.SessionID)
	h.hub.register(c)
	defer h.hub.unregister(c)

	ctx, cancel := context.WithCancel(context.WithoutCancel(r.Context()))
	defer cancel()
	go h.writeLoop(ctx, cancel, conn, c)
	h.readLoop(ctx, conn, c)
	return nil
}

func (h *Handler) writeLoop(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, c *Client) {
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.overflow:
			if err := conn.Close(websocket.StatusInternalError, "send buffer overflow"); err != nil {
				h.log.Debug("ws close after overflow", "err", err)
			}
			return
		case msg := <-c.send:
			wctx, wcancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Write(wctx, websocket.MessageText, msg)
			wcancel()
			if err != nil {
				h.log.Debug("ws write", "err", err)
				return
			}
		}
	}
}

func (h *Handler) readLoop(ctx context.Context, conn *websocket.Conn, c *Client) {
	defer func() {
		if err := conn.CloseNow(); err != nil {
			h.log.Debug("ws close", "err", err)
		}
	}()
	for {
		rctx, rcancel := context.WithTimeout(ctx, readTimeout)
		_, raw, err := conn.Read(rctx)
		rcancel()
		if err != nil {
			return
		}
		var in inbound
		if err := json.Unmarshal(raw, &in); err != nil {
			h.replyError(c, "validation_failed", "bad message")
			continue
		}
		switch in.Type {
		case typeClientPing:
			h.replyPong(c)
		case typeClientSubscribe:
			var d subscribeData
			if err := json.Unmarshal(in.Data, &d); err != nil || d.HallID == "" {
				h.replyError(c, "validation_failed", "hall_id required")
				continue
			}
			h.subscribe(ctx, c, d.HallID)
		default:
			h.replyError(c, "validation_failed", "unknown type")
		}
	}
}

func (h *Handler) subscribe(ctx context.Context, c *Client, hallID string) {
	base := h.hub.beginSubscribe(c, hallID)
	state, err := h.snapshots.HallState(ctx, c.venueID, hallID, h.hub.Online(c.venueID, hallID), h.hub.now())
	if err != nil {
		c.abortSubscribe()
		code, _ := httpx.ErrorCode(err)
		if code == "internal" {
			h.log.Error("ws snapshot", "hall_id", hallID, "err", err)
		}
		h.replyError(c, code, "subscribe failed")
		return
	}
	msg, err := h.hub.direct(c.venueID, &hallID, TypeSnapshot, state, base)
	if err != nil {
		c.abortSubscribe()
		h.log.Error("ws snapshot", "err", err)
		return
	}
	c.finishSubscribe(msg, base)
}

func (h *Handler) replyPong(c *Client) {
	msg, err := h.hub.direct(c.venueID, nil, TypePong, PongData{ServerTime: h.hub.now().UTC()}, h.hub.Seq(c.venueID))
	if err != nil {
		h.log.Error("ws pong", "err", err)
		return
	}
	c.reply(msg)
}

func (h *Handler) replyError(c *Client, code, message string) {
	msg, err := h.hub.direct(c.venueID, nil, TypeError, ErrorData{Code: code, Message: message}, h.hub.Seq(c.venueID))
	if err != nil {
		h.log.Error("ws error reply", "err", err)
		return
	}
	c.reply(msg)
}
