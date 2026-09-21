package push

import (
	"context"
	"time"
)

// Message is the Expo push payload, tech.md section 10.
type Message struct {
	To         string `json:"to"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Sound      string `json:"sound"`
	Priority   string `json:"priority"`
	CategoryID string `json:"categoryId"`
	Data       Data   `json:"data"`
}

type Data struct {
	EventID  string    `json:"event_id"`
	HookahID string    `json:"hookah_id"`
	TableID  string    `json:"table_id"`
	HallID   string    `json:"hall_id"`
	Kind     string    `json:"kind"`
	DueAt    time.Time `json:"due_at"`
}

type Status string

const (
	StatusSent         Status = "sent"
	StatusFailed       Status = "failed"
	StatusInvalidToken Status = "invalid_token"
	StatusCancelled    Status = "cancelled"
)

// Result is the outcome of one message, in the order of the input.
type Result struct {
	Status  Status
	Receipt string
	Error   string
}

// Sender delivers a batch. A returned error means the whole batch failed in
// transport; per-message problems are reported in Result.
type Sender interface {
	Send(ctx context.Context, msgs []Message) ([]Result, error)
}
