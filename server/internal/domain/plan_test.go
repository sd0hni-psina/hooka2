package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestMaterializePlanMatchesSharedFixture(t *testing.T) {
	var fx struct {
		Cases []struct {
			Name      string    `json:"name"`
			StartedAt time.Time `json:"started_at"`
			Plan      Plan      `json:"plan"`
			Expected  []Event   `json:"expected"`
		} `json:"cases"`
	}
	readFixture(t, "plan_cases.json", &fx)
	require.NotEmpty(t, fx.Cases)

	for _, c := range fx.Cases {
		t.Run(c.Name, func(t *testing.T) {
			require.NoError(t, ValidatePlan(c.Plan), "fixture plans must be valid")
			got := MaterializePlan(c.Plan, c.StartedAt)
			require.Len(t, got, len(c.Expected))
			for i := range c.Expected {
				require.Equal(t, c.Expected[i].Seq, got[i].Seq)
				require.Equal(t, c.Expected[i].Kind, got[i].Kind)
				require.Equal(t, c.Expected[i].Title, got[i].Title)
				require.True(t, c.Expected[i].DueAt.Equal(got[i].DueAt), "event %d: want %s got %s", i, c.Expected[i].DueAt, got[i].DueAt)
			}
		})
	}
}

func TestDefaultPlanIsValid(t *testing.T) {
	require.NoError(t, ValidatePlan(DefaultPlan()))
}

// genPlan draws plans that satisfy every constraint of ValidatePlan.
func genPlan(t *rapid.T) Plan {
	expireOffset := rapid.IntRange(61, 86400).Draw(t, "expire_offset")
	n := rapid.IntRange(0, 11).Draw(t, "extra_steps")
	kinds := []EventKind{KindCheck, KindShake, KindReplace, KindCustom}
	steps := make([]PlanStep, 0, n+1)
	for i := 0; i < n; i++ {
		s := PlanStep{
			Kind:      rapid.SampledFrom(kinds).Draw(t, "kind"),
			Title:     rapid.StringMatching(`[a-zа-я]{1,20}`).Draw(t, "title"),
			OffsetSec: rapid.IntRange(60, expireOffset-1).Draw(t, "offset"),
		}
		if rapid.Bool().Draw(t, "repeats") {
			r := rapid.IntRange(120, 86400).Draw(t, "repeat")
			s.RepeatEverySec = &r
		}
		steps = append(steps, s)
	}
	pos := rapid.IntRange(0, len(steps)).Draw(t, "expire_pos")
	steps = append(steps[:pos], append([]PlanStep{{Kind: KindExpire, Title: "end", OffsetSec: expireOffset}}, steps[pos:]...)...)
	grace := rapid.IntRange(30, 900).Draw(t, "grace")
	return Plan{
		Version: 1, Steps: steps,
		WarnBeforeSec:   rapid.IntRange(0, 300).Draw(t, "warn"),
		GraceSec:        grace,
		MissedAfterSec:  rapid.IntRange(grace+1, grace+3600).Draw(t, "missed"),
		PushRepeatSec:   120,
		PushMaxAttempts: 3,
	}
}

func TestMaterializePlanProperties(t *testing.T) {
	start := time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC)
	rapid.Check(t, func(t *rapid.T) {
		plan := genPlan(t)
		if err := ValidatePlan(plan); err != nil {
			t.Fatalf("generator produced invalid plan: %v", err)
		}
		events := MaterializePlan(plan, start)
		if len(events) == 0 {
			t.Fatal("valid plan produced no events")
		}

		expireOffset, _ := plan.ExpireOffset()
		expireAt := start.Add(time.Duration(expireOffset) * time.Second)
		last := events[len(events)-1]
		if last.Kind != KindExpire || !last.DueAt.Equal(expireAt) {
			t.Fatalf("last event must be expire at %s, got %s at %s", expireAt, last.Kind, last.DueAt)
		}

		expires := 0
		for i, e := range events {
			if e.Seq != i+1 {
				t.Fatalf("seq has a hole: index %d has seq %d", i, e.Seq)
			}
			if e.Kind == KindExpire {
				expires++
			}
			if e.DueAt.After(expireAt) {
				t.Fatalf("event after expire: %v", e)
			}
			if i > 0 && !events[i-1].DueAt.Before(e.DueAt) {
				t.Fatalf("due_at not strictly increasing at %d", i)
			}
		}
		if expires != 1 {
			t.Fatalf("want exactly one expire, got %d", expires)
		}

		again := MaterializePlan(plan, start)
		if len(again) != len(events) {
			t.Fatal("not deterministic")
		}
		for i := range events {
			if again[i] != events[i] {
				t.Fatalf("not deterministic at %d", i)
			}
		}
	})
}

func TestValidatePlan(t *testing.T) {
	mutate := func(f func(p *Plan)) Plan {
		p := DefaultPlan()
		f(&p)
		return p
	}
	cases := []struct {
		name   string
		plan   Plan
		field  string
		reason string
	}{
		{"two expire steps are rejected", mutate(func(p *Plan) {
			p.Steps = append(p.Steps, PlanStep{Kind: KindExpire, Title: "x", OffsetSec: 5000})
		}), "steps[4].kind", "multiple_expire"},
		{"missing expire is rejected", mutate(func(p *Plan) { p.Steps = p.Steps[:3] }), "steps", "missing_expire"},
		{"offset after expire is rejected", mutate(func(p *Plan) { p.Steps[0].OffsetSec = 4800 }), "steps[0].offset_sec", "offset_after_expire"},
		{"a ten second step is rejected", mutate(func(p *Plan) { p.Steps[0].OffsetSec = 10 }), "steps[0].offset_sec", "offset_out_of_range"},
		{"repeat below two minutes is rejected", mutate(func(p *Plan) { r := 119; p.Steps[1].RepeatEverySec = &r }), "steps[1].repeat_every_sec", "repeat_out_of_range"},
		{"repeating expire is rejected", mutate(func(p *Plan) { r := 600; p.Steps[3].RepeatEverySec = &r }), "steps[3].repeat_every_sec", "expire_repeats"},
		{"thirteen steps are rejected", mutate(func(p *Plan) {
			for len(p.Steps) < 13 {
				p.Steps = append([]PlanStep{{Kind: KindCheck, Title: "c", OffsetSec: 60}}, p.Steps...)
			}
		}), "steps", "steps_count_out_of_range"},
		{"title over 120 runes is rejected", mutate(func(p *Plan) {
			p.Steps[0].Title = strings.Repeat("я", 121)
		}), "steps[0].title", "title_length_out_of_range"},
		{"grace below 30 is rejected", mutate(func(p *Plan) { p.GraceSec = 29 }), "grace_sec", "grace_out_of_range"},
		{"warn above 300 is rejected", mutate(func(p *Plan) { p.WarnBeforeSec = 301 }), "warn_before_sec", "warn_out_of_range"},
		{"missed equal to grace is rejected", mutate(func(p *Plan) { p.MissedAfterSec = p.GraceSec }), "missed_after_sec", "missed_not_after_grace"},
		{"unknown kind is rejected", mutate(func(p *Plan) { p.Steps[0].Kind = "smoke" }), "steps[0].kind", "unknown_kind"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePlan(c.plan)
			require.ErrorIs(t, err, ErrPlanInvalid)
			var pe *PlanError
			require.ErrorAs(t, err, &pe)
			require.Equal(t, c.field, pe.Field)
			require.Equal(t, c.reason, pe.Reason)
		})
	}

	t.Run("title of exactly 120 runes passes", func(t *testing.T) {
		require.NoError(t, ValidatePlan(mutate(func(p *Plan) { p.Steps[0].Title = strings.Repeat("я", 120) })))
	})
}
