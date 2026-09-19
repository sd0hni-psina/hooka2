package hookah

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// payloads maps every event this package owns to the Go type behind it.
var payloads = map[string]func() any{
	"state.snapshot":     func() any { return &HallState{} },
	"hookah.started":     func() any { return &StartedData{} },
	"hookah.transferred": func() any { return &TransferredData{} },
	"hookah.expired":     func() any { return &ExpiredData{} },
	"hookah.finished":    func() any { return &FinishedData{} },
	"service.due":        func() any { return &DueData{} },
	"service.missed":     func() any { return &MissedData{} },
}

func TestEventPayloadsMatchSharedFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "fixtures", "ws_events.json"))
	require.NoError(t, err)
	var fx struct {
		Events []struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		} `json:"events"`
	}
	require.NoError(t, json.Unmarshal(raw, &fx))

	seen := map[string]bool{}
	for _, ev := range fx.Events {
		mk, ok := payloads[ev.Type]
		if !ok {
			continue
		}
		seen[ev.Type] = true
		t.Run(ev.Type, func(t *testing.T) {
			dec := json.NewDecoder(bytes.NewReader(ev.Data))
			dec.DisallowUnknownFields()
			payload := mk()
			require.NoError(t, dec.Decode(payload))
			out, err := json.Marshal(payload)
			require.NoError(t, err)
			require.JSONEq(t, string(ev.Data), string(out))
		})
	}
	for typ := range payloads {
		require.True(t, seen[typ], "fixture has no %s", typ)
	}
}
