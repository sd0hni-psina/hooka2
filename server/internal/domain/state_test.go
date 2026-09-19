package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckMutable(t *testing.T) {
	require.NoError(t, CheckMutable(HookahActive))
	require.NoError(t, CheckMutable(HookahExpired))
	require.ErrorIs(t, CheckMutable(HookahFinished), ErrHookahFinished)
}

func TestDecideConfirm(t *testing.T) {
	ev := func(id string, seq int, kind EventKind, state EventState) ServiceEvent {
		return ServiceEvent{ID: id, Seq: seq, Kind: kind, State: state}
	}
	all := []ServiceEvent{
		ev("check", 1, KindCheck, EventConfirmed),
		ev("shake1", 2, KindShake, EventDue),
		ev("replace1", 3, KindReplace, EventPending),
		ev("shake2", 4, KindShake, EventPending),
		ev("replace2", 5, KindReplace, EventPending),
		ev("missed", 6, KindCustom, EventMissed),
		ev("cancelled", 7, KindCustom, EventCancelled),
	}
	find := func(id string) ServiceEvent {
		for _, e := range all {
			if e.ID == id {
				return e
			}
		}
		t.Fatalf("no event %s", id)
		return ServiceEvent{}
	}

	cases := []struct {
		name   string
		hookah HookahState
		id     string
		want   ConfirmDecision
		err    error
	}{
		{"due event confirms now", HookahActive, "shake1", ConfirmNow, nil},
		{"due event on an expired hookah confirms now", HookahExpired, "shake1", ConfirmNow, nil},
		{"already confirmed event reports success without a write", HookahActive, "check", ConfirmAlreadyDone, nil},
		{"nearest pending of its kind confirms early", HookahActive, "replace1", ConfirmEarly, nil},
		{"pending shake confirms early while an earlier shake is due", HookahActive, "shake2", ConfirmEarly, nil},
		{"second pending of a kind is not actionable", HookahActive, "replace2", 0, ErrEventNotActionable},
		{"missed event is not actionable", HookahActive, "missed", 0, ErrEventNotActionable},
		{"cancelled event is not actionable", HookahActive, "cancelled", 0, ErrEventNotActionable},
		{"finished hookah rejects any confirmation", HookahFinished, "shake1", 0, ErrHookahFinished},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := DecideConfirm(c.hookah, find(c.id), all)
			if c.err != nil {
				require.ErrorIs(t, err, c.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.want, got)
		})
	}
}

func TestValidationErrorKeepsFirstReason(t *testing.T) {
	var v ValidationError
	require.NoError(t, v.Err())
	v.Add("email", ReasonRequired)
	v.Add("email", ReasonInvalid)
	require.ErrorIs(t, v.Err(), ErrValidation)
	require.Equal(t, map[string]string{"email": ReasonRequired}, v.Fields)
}
