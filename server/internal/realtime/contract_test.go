package realtime

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// decodeStrict unmarshals into v rejecting unknown fields, then re-encodes, so
// a fixture field missing from the Go type fails the test.
func decodeStrict(t *testing.T, raw json.RawMessage, v any) []byte {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(v))
	out, err := json.Marshal(v)
	require.NoError(t, err)
	return out
}

// payloads maps each event type to the Go type that produces its data.
// Slices add their types here as they add events.
var payloads = map[string]func() any{
	TypePong:  func() any { return &PongData{} },
	TypeError: func() any { return &ErrorData{} },
}

func TestEnvelopesMatchSharedFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "fixtures", "ws_events.json"))
	require.NoError(t, err)
	var fx struct {
		Events []json.RawMessage `json:"events"`
	}
	require.NoError(t, json.Unmarshal(raw, &fx))

	for _, ev := range fx.Events {
		var env struct {
			Envelope
			Data json.RawMessage `json:"data"`
		}
		roundTrip := decodeStrict(t, ev, &env)
		require.Equal(t, ProtocolVersion, env.V)
		require.NotEmpty(t, env.VenueID)
		var generic map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(roundTrip, &generic))
		for _, k := range []string{"v", "type", "seq", "ts", "venue_id", "hall_id", "data"} {
			require.Contains(t, generic, k, "%s envelope lacks %s", env.Type, k)
		}

		mk, ok := payloads[env.Type]
		if !ok {
			// Snapshot data is checked by the hookah package, which owns HallState.
			continue
		}
		t.Run(env.Type, func(t *testing.T) {
			got := decodeStrict(t, env.Data, mk())
			require.JSONEq(t, string(env.Data), string(got))
		})
	}
}
