package push

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// Fake validates every payload and records it instead of calling Expo.
// Malformed payloads fail loudly, so contract drift shows up in tests.
type Fake struct {
	log *slog.Logger

	mu   sync.Mutex
	sent []Message
	// Next overrides results for the following Send calls, one per call.
	Next []func(msgs []Message) ([]Result, error)
}

func NewFake(log *slog.Logger) *Fake { return &Fake{log: log} }

func (f *Fake) Send(ctx context.Context, msgs []Message) ([]Result, error) {
	for i, m := range msgs {
		if err := Validate(m); err != nil {
			return nil, fmt.Errorf("fake push: message %d: %w", i, err)
		}
	}
	f.mu.Lock()
	var next func([]Message) ([]Result, error)
	if len(f.Next) > 0 {
		next, f.Next = f.Next[0], f.Next[1:]
	}
	f.sent = append(f.sent, msgs...)
	f.mu.Unlock()

	if next != nil {
		return next(msgs)
	}
	out := make([]Result, len(msgs))
	for i, m := range msgs {
		f.log.InfoContext(ctx, "fake push", "to", m.To, "title", m.Title, "body", m.Body, "event_id", m.Data.EventID)
		out[i] = Result{Status: StatusSent, Receipt: "fake-" + m.Data.EventID}
	}
	return out, nil
}

func (f *Fake) Sent() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Message(nil), f.sent...)
}

// Validate checks the fields Expo and the app rely on.
func Validate(m Message) error {
	switch {
	case !strings.HasPrefix(m.To, "ExponentPushToken[") || !strings.HasSuffix(m.To, "]"):
		return fmt.Errorf("bad token %q", m.To)
	case m.Title == "" || m.Body == "":
		return fmt.Errorf("empty title or body")
	case m.Sound != "default" || m.Priority != "high" || m.CategoryID != "service_action":
		return fmt.Errorf("bad sound, priority or category")
	case m.Data.EventID == "" || m.Data.HookahID == "" || m.Data.TableID == "" || m.Data.HallID == "" || m.Data.Kind == "" || m.Data.DueAt.IsZero():
		return fmt.Errorf("incomplete data")
	}
	return nil
}
