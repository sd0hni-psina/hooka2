package staff_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
	"hookah/server/internal/staff"
)

func keys(v any, prefix string, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			*out = append(*out, prefix+"."+k)
			keys(child, prefix+"."+k, out)
		}
	case []any:
		if len(t) > 0 {
			keys(t[0], prefix+"[]", out)
		}
	}
}

func keySet(t *testing.T, raw []byte) []string {
	var v any
	require.NoError(t, json.Unmarshal(raw, &v))
	var out []string
	keys(v, "", &out)
	sort.Strings(out)
	return out
}

func TestMeMatchesSharedFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "fixtures", "me.json"))
	require.NoError(t, err)

	t.Run("go type round-trips the fixture exactly", func(t *testing.T) {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		var me staff.Me
		require.NoError(t, dec.Decode(&me))
		out, err := json.Marshal(me)
		require.NoError(t, err)
		require.JSONEq(t, string(raw), string(out))
	})

	t.Run("live response has the fixture shape", func(t *testing.T) {
		e := apitest.New(t, true)
		res := e.Do("GET", "/me", e.TokenFor(e.Fx.Masters[0]), nil)
		require.Equal(t, 200, res.Status)
		require.Equal(t, keySet(t, raw), keySet(t, res.Raw))
	})
}
