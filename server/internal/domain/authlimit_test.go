package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestLoginDecision(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		a       LoginAttempts
		retry   int
		limited bool
	}{
		{"no history passes", LoginAttempts{}, 0, false},
		{"nine attempts from one ip in a minute pass", LoginAttempts{IP: evenly(now, 9, 59, 1)}, 0, false},
		{"ten attempts from one ip in a minute limit until the oldest ages out", LoginAttempts{IP: evenly(now, 10, 50, 1)}, 10, true},
		{"ip attempts older than a minute do not count", LoginAttempts{IP: evenly(now, 10, 900, 61)}, 0, false},
		{"four wrong passwords for one email in fifteen minutes pass", LoginAttempts{EmailFailed: evenly(now, 4, 800, 1)}, 0, false},
		{"five wrong passwords for one email in fifteen minutes limit", LoginAttempts{EmailFailed: evenly(now, 5, 800, 1)}, 100, true},
		{"wrong passwords older than fifteen minutes do not count", LoginAttempts{EmailFailed: evenly(now, 5, 3000, 901)}, 0, false},
		{"the longer wait of the two limits wins", LoginAttempts{
			IP:          evenly(now, 10, 50, 1),
			EmailFailed: evenly(now, 5, 800, 1),
		}, 100, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			retry, limited := LoginDecision(c.a, now)
			require.Equal(t, c.limited, limited)
			require.Equal(t, c.retry, retry)
		})
	}
}

func TestRegisterDecision(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		ip      []time.Time
		retry   int
		limited bool
	}{
		{"no history passes", nil, 0, false},
		{"four registrations from one ip in an hour pass", evenly(now, 4, 3500, 10), 0, false},
		{"five registrations from one ip in an hour limit until the oldest ages out", evenly(now, 5, 3500, 10), 100, true},
		{"registrations older than an hour do not count", evenly(now, 5, 9000, 3601), 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			retry, limited := RegisterDecision(c.ip, now)
			require.Equal(t, c.limited, limited)
			require.Equal(t, c.retry, retry)
		})
	}
}

func TestAuthDecisionsAllowTheAttemptOnceTheWaitPasses(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	draw := func(t *rapid.T, name string, maxN int) []time.Time {
		n := rapid.IntRange(0, maxN).Draw(t, name+"_n")
		out := make([]time.Time, n)
		for i := range out {
			out[i] = now.Add(-time.Duration(rapid.IntRange(0, 4000).Draw(t, name+"_age")) * time.Second)
		}
		return out
	}
	rapid.Check(t, func(t *rapid.T) {
		login := LoginAttempts{IP: draw(t, "ip", 14), EmailFailed: draw(t, "email", 8)}
		if retry, limited := LoginDecision(login, now); limited {
			require.Positive(t, retry)
			_, still := LoginDecision(login, now.Add(time.Duration(retry)*time.Second))
			require.False(t, still, "login still limited after waiting %d s", retry)
		} else {
			require.Zero(t, retry)
		}
		reg := draw(t, "register", 8)
		if retry, limited := RegisterDecision(reg, now); limited {
			require.Positive(t, retry)
			_, still := RegisterDecision(reg, now.Add(time.Duration(retry)*time.Second))
			require.False(t, still, "register still limited after waiting %d s", retry)
		} else {
			require.Zero(t, retry)
		}
	})
}
