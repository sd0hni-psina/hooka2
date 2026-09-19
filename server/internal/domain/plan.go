package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type EventKind string

const (
	KindCheck   EventKind = "check"
	KindShake   EventKind = "shake"
	KindReplace EventKind = "replace"
	KindExpire  EventKind = "expire"
	KindCustom  EventKind = "custom"
)

// priority resolves collisions in one second: expire > replace > shake > check > custom.
func (k EventKind) priority() int {
	switch k {
	case KindExpire:
		return 5
	case KindReplace:
		return 4
	case KindShake:
		return 3
	case KindCheck:
		return 2
	case KindCustom:
		return 1
	}
	return 0
}

func (k EventKind) Valid() bool { return k.priority() > 0 }

type PlanStep struct {
	Kind           EventKind `json:"kind"`
	Title          string    `json:"title"`
	OffsetSec      int       `json:"offset_sec"`
	RepeatEverySec *int      `json:"repeat_every_sec,omitempty"`
}

type Plan struct {
	Version         int        `json:"version"`
	Steps           []PlanStep `json:"steps"`
	WarnBeforeSec   int        `json:"warn_before_sec"`
	GraceSec        int        `json:"grace_sec"`
	MissedAfterSec  int        `json:"missed_after_sec"`
	PushRepeatSec   int        `json:"push_repeat_sec"`
	PushMaxAttempts int        `json:"push_max_attempts"`
}

// Event is one materialized service task of a hookah.
type Event struct {
	Seq   int       `json:"seq"`
	Kind  EventKind `json:"kind"`
	Title string    `json:"title"`
	DueAt time.Time `json:"due_at"`
}

func intPtr(v int) *int { return &v }

// DefaultPlan is the plan every new venue starts with (tech.md section 7).
func DefaultPlan() Plan {
	return Plan{
		Version: 1,
		Steps: []PlanStep{
			{Kind: KindCheck, Title: "Проверьте кальян и узнайте у гостя, всё ли хорошо", OffsetSec: 300},
			{Kind: KindShake, Title: "Стряхните угли", OffsetSec: 900, RepeatEverySec: intPtr(900)},
			{Kind: KindReplace, Title: "Замените угли", OffsetSec: 1800, RepeatEverySec: intPtr(1800)},
			{Kind: KindExpire, Title: "Кальян отработал ресурс", OffsetSec: 4800},
		},
		WarnBeforeSec:   60,
		GraceSec:        120,
		MissedAfterSec:  600,
		PushRepeatSec:   120,
		PushMaxAttempts: 3,
	}
}

// ExpireOffset returns the offset of the single expire step.
func (p Plan) ExpireOffset() (int, bool) {
	for _, s := range p.Steps {
		if s.Kind == KindExpire {
			return s.OffsetSec, true
		}
	}
	return 0, false
}

// ValidatePlan checks the constraints of tech.md section 7.
func ValidatePlan(p Plan) error {
	fail := func(field, reason string) error { return &PlanError{Field: field, Reason: reason} }

	if p.Version != 1 {
		return fail("version", "unsupported_version")
	}
	if len(p.Steps) < 1 || len(p.Steps) > 12 {
		return fail("steps", "steps_count_out_of_range")
	}
	expireIdx := -1
	for i, s := range p.Steps {
		field := fmt.Sprintf("steps[%d]", i)
		if !s.Kind.Valid() {
			return fail(field+".kind", "unknown_kind")
		}
		if n := utf8.RuneCountInString(strings.TrimSpace(s.Title)); n == 0 || n > 120 {
			return fail(field+".title", "title_length_out_of_range")
		}
		if s.OffsetSec < 60 || s.OffsetSec > 86400 {
			return fail(field+".offset_sec", "offset_out_of_range")
		}
		if s.RepeatEverySec != nil && (*s.RepeatEverySec < 120 || *s.RepeatEverySec > 86400) {
			return fail(field+".repeat_every_sec", "repeat_out_of_range")
		}
		if s.Kind == KindExpire {
			if expireIdx >= 0 {
				return fail(field+".kind", "multiple_expire")
			}
			expireIdx = i
			if s.RepeatEverySec != nil {
				return fail(field+".repeat_every_sec", "expire_repeats")
			}
		}
	}
	if expireIdx < 0 {
		return fail("steps", "missing_expire")
	}
	expireOffset := p.Steps[expireIdx].OffsetSec
	for i, s := range p.Steps {
		if i != expireIdx && s.OffsetSec >= expireOffset {
			return fail(fmt.Sprintf("steps[%d].offset_sec", i), "offset_after_expire")
		}
	}
	if p.GraceSec < 30 || p.GraceSec > 900 {
		return fail("grace_sec", "grace_out_of_range")
	}
	if p.WarnBeforeSec < 0 || p.WarnBeforeSec > 300 {
		return fail("warn_before_sec", "warn_out_of_range")
	}
	if p.MissedAfterSec <= p.GraceSec {
		return fail("missed_after_sec", "missed_not_after_grace")
	}
	return nil
}

// MaterializePlan expands a valid plan into the full list of service events
// of a hookah started at startedAt. The plan must pass ValidatePlan; a plan
// without an expire step has no upper bound and yields nil.
func MaterializePlan(plan Plan, startedAt time.Time) []Event {
	expireOffset, ok := plan.ExpireOffset()
	if !ok {
		return nil
	}

	type candidate struct {
		offset int
		step   int
		kind   EventKind
		title  string
	}
	// One slot per second: the winner is the higher priority kind, then the earlier step.
	slots := map[int]candidate{}
	put := func(c candidate) {
		cur, taken := slots[c.offset]
		if !taken || c.kind.priority() > cur.kind.priority() ||
			(c.kind.priority() == cur.kind.priority() && c.step < cur.step) {
			slots[c.offset] = c
		}
	}

	for i, s := range plan.Steps {
		if s.Kind == KindExpire {
			put(candidate{offset: s.OffsetSec, step: i, kind: s.Kind, title: s.Title})
			continue
		}
		for off := s.OffsetSec; off < expireOffset; {
			put(candidate{offset: off, step: i, kind: s.Kind, title: s.Title})
			if s.RepeatEverySec == nil || *s.RepeatEverySec <= 0 {
				break
			}
			off += *s.RepeatEverySec
		}
	}

	offsets := make([]int, 0, len(slots))
	for off := range slots {
		offsets = append(offsets, off)
	}
	sort.Ints(offsets)

	events := make([]Event, len(offsets))
	for i, off := range offsets {
		c := slots[off]
		events[i] = Event{Seq: i + 1, Kind: c.kind, Title: c.title, DueAt: secondsAfter(startedAt, off)}
	}
	return events
}
