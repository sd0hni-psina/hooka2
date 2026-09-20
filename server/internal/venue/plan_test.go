package venue_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
	"hookah/server/internal/domain"
	"hookah/server/internal/venue"
)

func plan(mutate func(p *domain.Plan)) map[string]any {
	p := domain.DefaultPlan()
	mutate(&p)
	raw, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}

func TestServicePlan(t *testing.T) {
	t.Run("admin shortens the shake interval and only new hookahs follow it", func(t *testing.T) {
		e := apitest.New(t, true)
		e.Exec(`update staff set role = 'admin' where id = $1`, e.Fx.Masters[0].StaffID)
		promoted := e.Fx.Masters[0]
		promoted.Role = domain.RoleAdmin
		admin := e.TokenFor(promoted)
		ws, _ := e.Subscribe(e.TokenFor(e.Fx.Masters[1]), e.Fx.HallIDs[0])

		before := e.Do("GET", "/venues/current/service-plan", admin, nil)
		require.Equal(t, 200, before.Status)
		require.EqualValues(t, 900, before.Body["steps"].([]any)[1].(map[string]any)["offset_sec"])

		res := e.Do("PUT", "/venues/current/service-plan", admin, plan(func(p *domain.Plan) {
			p.Steps[1].OffsetSec = 720
			repeat := 720
			p.Steps[1].RepeatEverySec = &repeat
		}))
		require.Equal(t, 200, res.Status, string(res.Raw))
		require.EqualValues(t, 720, res.Body["steps"].([]any)[1].(map[string]any)["offset_sec"])

		m := ws.Until("plan.updated")
		require.Nil(t, m.HallID, "the plan is venue wide")
		var data venue.PlanUpdated
		require.NoError(t, json.Unmarshal(m.Data, &data))
		require.Equal(t, 720, data.Plan.Steps[1].OffsetSec)

		// The seeded hookah keeps the plan it started with.
		var snapshot domain.Plan
		require.NoError(t, e.Pool.QueryRow(context.Background(),
			`select plan_snapshot from hookahs where id = $1`, e.Fx.BusyHookahID).Scan(&snapshot))
		require.Equal(t, 900, snapshot.Steps[1].OffsetSec)

		// A hookah started now runs on the new plan.
		started := e.Do("POST", "/tables/"+e.Fx.TableIDs["1"]+"/hookah", admin, nil,
			"Idempotency-Key", "11111111-1111-4111-8111-111111111111")
		require.Equal(t, 200, started.Status, string(started.Raw))
		var h domain.Hookah
		started.Decode(t, &h)
		require.True(t, h.Events[1].DueAt.Equal(h.StartedAt.Add(12*time.Minute)), "the shake moved to minute 12")
	})

	t.Run("a plan that breaks the rules is rejected with a reason", func(t *testing.T) {
		e := apitest.New(t, true)
		owner := e.TokenFor(e.Fx.Owner)
		cases := []struct {
			name   string
			body   map[string]any
			field  string
			reason string
		}{
			{"two expire steps", plan(func(p *domain.Plan) {
				p.Steps = append(p.Steps, domain.PlanStep{Kind: domain.KindExpire, Title: "ещё конец", OffsetSec: 5000})
			}), "steps[4].kind", "multiple_expire"},
			{"a step after expire", plan(func(p *domain.Plan) { p.Steps[0].OffsetSec = 5000 }), "steps[0].offset_sec", "offset_after_expire"},
			{"a ten second step", plan(func(p *domain.Plan) { p.Steps[0].OffsetSec = 10 }), "steps[0].offset_sec", "offset_out_of_range"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				res := e.Do("PUT", "/venues/current/service-plan", owner, c.body)
				require.Equal(t, 422, res.Status)
				require.Equal(t, "plan_invalid", res.Code())
				require.Equal(t, c.reason, res.Details()["reason"])
				require.Equal(t, c.field, res.Details()["field"])
			})
		}
		// The stored plan did not change.
		res := e.Do("GET", "/venues/current/service-plan", owner, nil)
		require.EqualValues(t, 300, res.Body["steps"].([]any)[0].(map[string]any)["offset_sec"])
	})

	t.Run("a master may neither read nor write the plan", func(t *testing.T) {
		e := apitest.New(t, true)
		master := e.TokenFor(e.Fx.Masters[0])
		require.Equal(t, 403, e.Do("GET", "/venues/current/service-plan", master, nil).Status)
		require.Equal(t, 403, e.Do("PUT", "/venues/current/service-plan", master, plan(func(*domain.Plan) {})).Status)
	})
}
