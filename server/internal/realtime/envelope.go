package realtime

import (
	"encoding/json"
	"time"
)

const ProtocolVersion = 1

// Server to client event types, tech.md section 9.
const (
	TypeSnapshot        = "state.snapshot"
	TypeHookahStarted   = "hookah.started"
	TypeHookahTransfer  = "hookah.transferred"
	TypeHookahExpired   = "hookah.expired"
	TypeHookahFinished  = "hookah.finished"
	TypeServiceDue      = "service.due"
	TypeServiceClaimed  = "service.claimed"
	TypeServiceConfirm  = "service.confirmed"
	TypeServiceMissed   = "service.missed"
	TypePlanUpdated     = "plan.updated"
	TypeLayoutUpdated   = "layout.updated"
	TypePong            = "pong"
	TypeError           = "error"
	typeClientSubscribe = "subscribe"
	typeClientPing      = "ping"
)

type Envelope struct {
	V       int       `json:"v"`
	Type    string    `json:"type"`
	Seq     int64     `json:"seq"`
	TS      time.Time `json:"ts"`
	VenueID string    `json:"venue_id"`
	HallID  *string   `json:"hall_id"`
	Data    any       `json:"data"`
}

type inbound struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type subscribeData struct {
	HallID string `json:"hall_id"`
}

type PongData struct {
	ServerTime time.Time `json:"server_time"`
}

type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
