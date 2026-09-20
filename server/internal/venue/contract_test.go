package venue

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlanUpdatedMatchesSharedFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "fixtures", "ws_events.json"))
	require.NoError(t, err)
	var fx struct {
		Events []struct {
			Type   string          `json:"type"`
			HallID *string         `json:"hall_id"`
			Data   json.RawMessage `json:"data"`
		} `json:"events"`
	}
	require.NoError(t, json.Unmarshal(raw, &fx))

	found := false
	for _, ev := range fx.Events {
		if ev.Type != "plan.updated" {
			continue
		}
		found = true
		require.Nil(t, ev.HallID, "plan.updated is venue wide")
		dec := json.NewDecoder(bytes.NewReader(ev.Data))
		dec.DisallowUnknownFields()
		var payload PlanUpdated
		require.NoError(t, dec.Decode(&payload))
		out, err := json.Marshal(payload)
		require.NoError(t, err)
		require.JSONEq(t, string(ev.Data), string(out))
	}
	require.True(t, found, "fixture has no plan.updated")
}
