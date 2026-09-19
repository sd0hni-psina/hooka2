package layout

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLayoutUpdatedMatchesSharedFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "fixtures", "ws_events.json"))
	require.NoError(t, err)
	var fx struct {
		Events []struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		} `json:"events"`
	}
	require.NoError(t, json.Unmarshal(raw, &fx))

	found := false
	for _, ev := range fx.Events {
		if ev.Type != "layout.updated" {
			continue
		}
		found = true
		dec := json.NewDecoder(bytes.NewReader(ev.Data))
		dec.DisallowUnknownFields()
		var payload LayoutUpdated
		require.NoError(t, dec.Decode(&payload))
		out, err := json.Marshal(payload)
		require.NoError(t, err)
		require.JSONEq(t, string(ev.Data), string(out))
	}
	require.True(t, found, "fixture has no layout.updated")
}
