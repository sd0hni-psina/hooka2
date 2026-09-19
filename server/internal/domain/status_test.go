package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestTableStatusMatchesSharedFixture(t *testing.T) {
	var fx struct {
		Plan  StatusPlan `json:"plan"`
		Now   time.Time  `json:"now"`
		Cases []struct {
			Name   string `json:"name"`
			Hookah *struct {
				State  HookahState   `json:"state"`
				Events []StatusEvent `json:"events"`
			} `json:"hookah"`
			Expected Status `json:"expected"`
		} `json:"cases"`
	}
	readFixture(t, "status_cases.json", &fx)
	require.NotEmpty(t, fx.Cases)

	for _, c := range fx.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var got Status
			if c.Hookah == nil {
				got = TableStatus(nil, nil, fx.Plan, fx.Now)
			} else {
				state := c.Hookah.State
				got = TableStatus(&state, c.Hookah.Events, fx.Plan, fx.Now)
			}
			require.Equal(t, c.Expected, got)
		})
	}
}

func TestTableStatusNeverFreeForLiveHookah(t *testing.T) {
	base := time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC)
	rapid.Check(t, func(t *rapid.T) {
		state := rapid.SampledFrom([]HookahState{HookahActive, HookahExpired}).Draw(t, "state")
		n := rapid.IntRange(0, 20).Draw(t, "events")
		events := make([]StatusEvent, n)
		for i := range events {
			events[i] = StatusEvent{
				DueAt: base.Add(time.Duration(rapid.IntRange(-7200, 7200).Draw(t, "due")) * time.Second),
				State: rapid.SampledFrom([]EventState{EventPending, EventDue, EventConfirmed, EventMissed, EventCancelled}).Draw(t, "event_state"),
			}
		}
		plan := StatusPlan{WarnBeforeSec: rapid.IntRange(0, 300).Draw(t, "warn"), GraceSec: rapid.IntRange(30, 900).Draw(t, "grace")}
		if got := TableStatus(&state, events, plan, base); got == StatusFree {
			t.Fatalf("live hookah in state %s is free", state)
		}
	})
}
