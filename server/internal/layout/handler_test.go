package layout_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
	"hookah/server/internal/domain"
	"hookah/server/internal/layout"
)

func table(label string, x, y int) map[string]any {
	return map[string]any{"label": label, "shape": "rect", "x": x, "y": y, "w": 8, "h": 6, "rotation": 0, "seats": 4}
}

func TestHallLayoutFlow(t *testing.T) {
	e := apitest.New(t, true)
	owner := e.TokenFor(e.Fx.Owner)
	master := e.TokenFor(e.Fx.Masters[0])

	res := e.Do("POST", "/halls", owner, map[string]any{"name": "Веранда", "sort_order": 5})
	require.Equal(t, 200, res.Status, string(res.Raw))
	var hall domain.Hall
	res.Decode(t, &hall)

	ws, snap := e.Subscribe(master, hall.ID)
	require.Contains(t, string(snap.Data), `"tables":[]`)

	ids := make([]string, 0, 18)
	for i := 1; i <= 18; i++ {
		res := e.Do("POST", "/halls/"+hall.ID+"/tables", owner, table(fmt.Sprint(i), (i-1)%6*16, (i-1)/6*20))
		require.Equal(t, 200, res.Status, string(res.Raw))
		ids = append(ids, res.Body["id"].(string))
	}

	t.Run("master sees each new table through layout.updated", func(t *testing.T) {
		var last layout.LayoutUpdated
		for range 18 {
			m := ws.Until("layout.updated")
			require.Equal(t, hall.ID, *m.HallID)
			require.NoError(t, json.Unmarshal(m.Data, &last))
		}
		require.Len(t, last.Tables, 18)
	})

	t.Run("dragging 18 tables saves in one batch request", func(t *testing.T) {
		batch := make([]map[string]any, len(ids))
		for i, id := range ids {
			batch[i] = map[string]any{"id": id, "x": 90 - i*5, "y": 2 + i*5%90, "w": 8, "h": 6, "rotation": 90}
		}
		res := e.Do("PUT", "/halls/"+hall.ID+"/layout", owner, map[string]any{"tables": batch})
		require.Equal(t, 200, res.Status, string(res.Raw))
		var tables []domain.Table
		res.Decode(t, &tables)
		require.Len(t, tables, 18)

		m := ws.Until("layout.updated")
		var upd layout.LayoutUpdated
		require.NoError(t, json.Unmarshal(m.Data, &upd))
		byID := map[string]domain.Table{}
		for _, tb := range upd.Tables {
			byID[tb.ID] = tb
		}
		require.Equal(t, 90, byID[ids[0]].X)
		require.Equal(t, 90, byID[ids[17]].Rotation)
	})

	t.Run("layout survives a fresh snapshot", func(t *testing.T) {
		_, snap := e.Subscribe(master, hall.ID)
		var st struct {
			Tables []domain.Table `json:"tables"`
		}
		require.NoError(t, json.Unmarshal(snap.Data, &st))
		require.Len(t, st.Tables, 18)
		for _, tb := range st.Tables {
			require.Equal(t, 90, tb.Rotation)
		}
	})

	t.Run("duplicate label in the hall is validation_failed", func(t *testing.T) {
		res := e.Do("POST", "/halls/"+hall.ID+"/tables", owner, table("7", 0, 90))
		require.Equal(t, 400, res.Status)
		require.Equal(t, map[string]any{"label": "taken"}, res.Details()["fields"])

		res = e.Do("PATCH", "/tables/"+ids[0], owner, map[string]any{"label": "2"})
		require.Equal(t, map[string]any{"label": "taken"}, res.Details()["fields"])
	})

	t.Run("archiving frees the label and restoring checks it again", func(t *testing.T) {
		require.Equal(t, 204, e.Do("DELETE", "/tables/"+ids[6], owner, nil).Status)
		again := e.Do("POST", "/halls/"+hall.ID+"/tables", owner, table("7", 0, 90))
		require.Equal(t, 200, again.Status, string(again.Raw))

		restore := e.Do("PATCH", "/tables/"+ids[6], owner, map[string]any{"archived": false})
		require.Equal(t, 400, restore.Status)
		require.Equal(t, map[string]any{"label": "taken"}, restore.Details()["fields"])

		restore = e.Do("PATCH", "/tables/"+ids[6], owner, map[string]any{"archived": false, "label": "7а"})
		require.Equal(t, 200, restore.Status, string(restore.Raw))
		require.Equal(t, "7а", restore.Body["label"])
	})

	t.Run("batch with one bad table changes nothing", func(t *testing.T) {
		res := e.Do("PUT", "/halls/"+hall.ID+"/layout", owner, map[string]any{"tables": []map[string]any{
			{"id": ids[0], "x": 1, "y": 1, "w": 8, "h": 6, "rotation": 0},
			{"id": ids[1], "x": 95, "y": 1, "w": 8, "h": 6, "rotation": 0},
		}})
		require.Equal(t, 400, res.Status)
		require.Equal(t, map[string]any{"tables[1].x": "out_of_bounds"}, res.Details()["fields"])
		require.Equal(t, 1, e.Count(`tables where id = $1 and x = 90`, ids[0]))
	})

	t.Run("batch with a table of another hall is not_found", func(t *testing.T) {
		res := e.Do("PUT", "/halls/"+hall.ID+"/layout", owner, map[string]any{"tables": []map[string]any{
			{"id": e.Fx.TableIDs["15"], "x": 1, "y": 1, "w": 8, "h": 6, "rotation": 0},
		}})
		require.Equal(t, 404, res.Status)
	})

	t.Run("empty hall is deleted", func(t *testing.T) {
		res := e.Do("POST", "/halls", owner, map[string]any{"name": "Временный", "sort_order": 9})
		id := res.Body["id"].(string)
		e.Do("POST", "/halls/"+id+"/tables", owner, table("1", 0, 0))
		require.Equal(t, 204, e.Do("DELETE", "/halls/"+id, owner, nil).Status)
		require.Equal(t, 0, e.Count(`tables where hall_id = $1`, id))
	})
}

func TestLayoutGuards(t *testing.T) {
	e := apitest.New(t, true)
	owner := e.TokenFor(e.Fx.Owner)
	master := e.TokenFor(e.Fx.Masters[0])
	hall1 := e.Fx.HallIDs[0]

	t.Run("table with a live hookah is not archived", func(t *testing.T) {
		res := e.Do("DELETE", "/tables/"+e.Fx.TableIDs["3"], owner, nil)
		require.Equal(t, 409, res.Status)
		require.Equal(t, "table_busy", res.Code())
		res = e.Do("PATCH", "/tables/"+e.Fx.TableIDs["3"], owner, map[string]any{"archived": true})
		require.Equal(t, "table_busy", res.Code())
	})

	t.Run("hall with a live hookah is not deleted", func(t *testing.T) {
		res := e.Do("DELETE", "/halls/"+hall1, owner, nil)
		require.Equal(t, 409, res.Status)
		require.Equal(t, "table_busy", res.Code())
	})

	t.Run("master gets forbidden on every layout endpoint", func(t *testing.T) {
		tableID := e.Fx.TableIDs["1"]
		calls := []apitest.Response{
			e.Do("POST", "/halls", master, map[string]any{"name": "x", "sort_order": 0}),
			e.Do("PATCH", "/halls/"+hall1, master, map[string]any{"name": "x"}),
			e.Do("DELETE", "/halls/"+hall1, master, nil),
			e.Do("POST", "/halls/"+hall1+"/tables", master, table("99", 0, 0)),
			e.Do("PATCH", "/tables/"+tableID, master, map[string]any{"x": 1}),
			e.Do("DELETE", "/tables/"+tableID, master, nil),
			e.Do("PUT", "/halls/"+hall1+"/layout", master, map[string]any{"tables": []map[string]any{{"id": tableID, "x": 1, "y": 1, "w": 5, "h": 5, "rotation": 0}}}),
		}
		for i, res := range calls {
			require.Equal(t, 403, res.Status, "call %d", i)
			require.Equal(t, "forbidden", res.Code(), "call %d", i)
		}
	})

	t.Run("hall names are unique ignoring case", func(t *testing.T) {
		res := e.Do("POST", "/halls", owner, map[string]any{"name": "основной зал", "sort_order": 0})
		require.Equal(t, 400, res.Status)
		require.Equal(t, map[string]any{"name": "taken"}, res.Details()["fields"])
	})

	t.Run("unknown hall and malformed id are not_found", func(t *testing.T) {
		require.Equal(t, 404, e.Do("PATCH", "/halls/00000000-0000-4000-8000-000000000000", owner, map[string]any{"name": "x"}).Status)
		require.Equal(t, 404, e.Do("DELETE", "/tables/15", owner, nil).Status)
	})

	t.Run("table outside the grid lists the field", func(t *testing.T) {
		res := e.Do("POST", "/halls/"+hall1+"/tables", owner, map[string]any{"label": "50", "shape": "circle", "x": 95, "y": 0, "w": 10, "h": 10, "rotation": 0, "seats": 4})
		require.Equal(t, 400, res.Status)
		require.Equal(t, map[string]any{"x": "out_of_bounds"}, res.Details()["fields"])
	})
}
