package domain

import "time"

type Status string

const (
	StatusFree   Status = "free"
	StatusGreen  Status = "green"
	StatusYellow Status = "yellow"
	StatusRed    Status = "red"
)

// StatusPlan holds the plan fields that affect the table color.
type StatusPlan struct {
	WarnBeforeSec int `json:"warn_before_sec"`
	GraceSec      int `json:"grace_sec"`
}

func (p Plan) StatusPlan() StatusPlan {
	return StatusPlan{WarnBeforeSec: p.WarnBeforeSec, GraceSec: p.GraceSec}
}

// StatusEvent is the part of a service event that affects the table color.
type StatusEvent struct {
	DueAt time.Time  `json:"due_at"`
	State EventState `json:"state"`
}

// TableStatus computes the table color, tech.md section 7. The first matching
// rule wins. A nil hookah means the table is free.
func TableStatus(hookah *HookahState, events []StatusEvent, plan StatusPlan, now time.Time) Status {
	if hookah == nil || *hookah == HookahFinished {
		return StatusFree
	}
	for _, e := range events {
		if e.State == EventDue && !now.Before(secondsAfter(e.DueAt, plan.GraceSec)) {
			return StatusRed
		}
	}
	if *hookah == HookahExpired {
		return StatusRed
	}
	for _, e := range events {
		if e.State == EventDue {
			return StatusYellow
		}
	}
	for _, e := range events {
		if e.State == EventPending && e.DueAt.Sub(now) <= time.Duration(plan.WarnBeforeSec)*time.Second {
			return StatusYellow
		}
	}
	return StatusGreen
}

// HookahStatus is TableStatus for a loaded hookah.
func HookahStatus(h *Hookah, plan StatusPlan, now time.Time) Status {
	if h == nil {
		return TableStatus(nil, nil, plan, now)
	}
	events := make([]StatusEvent, len(h.Events))
	for i, e := range h.Events {
		events[i] = StatusEvent{DueAt: e.DueAt, State: e.State}
	}
	state := h.State
	return TableStatus(&state, events, plan, now)
}
