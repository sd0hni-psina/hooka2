package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestJoinRetryAfter(t *testing.T) {
	now := time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC)
	ago := func(sec ...int) []time.Time {
		out := make([]time.Time, len(sec))
		for i, s := range sec {
			out[i] = now.Add(-time.Duration(s) * time.Second)
		}
		return out
	}
	cases := []struct {
		name     string
		attempts []time.Time
		retry    int
		limited  bool
	}{
		{"no attempts pass", nil, 0, false},
		{"fifth attempt within a minute passes", ago(50, 40, 30, 20), 0, false},
		{"sixth attempt within a minute is limited until the oldest ages out", ago(50, 40, 30, 20, 10), 10, true},
		{"attempts older than a minute do not count for the minute window", ago(61, 70, 80, 90, 100), 0, false},
		{"twenty attempts in an hour limit until the oldest ages out", spread(now, 20, 3000), 600, true},
		{"nineteen attempts in an hour pass", spread(now, 19, 3000), 0, false},
		{"both windows exceeded take the longer wait", append(spread(now, 15, 3000), ago(50, 40, 30, 20, 10)...), 600, true},
		{"retry rounds up to whole seconds", []time.Time{now.Add(-59500 * time.Millisecond), now, now, now, now}, 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			retry, limited := JoinRetryAfter(c.attempts, now)
			require.Equal(t, c.limited, limited)
			require.Equal(t, c.retry, retry)
		})
	}
}

// spread places n attempts evenly from spanSec seconds ago up to 60 seconds ago.
func spread(now time.Time, n, spanSec int) []time.Time {
	out := make([]time.Time, n)
	step := (spanSec - 61) / max(n-1, 1)
	for i := range out {
		out[i] = now.Add(-time.Duration(spanSec-i*step) * time.Second)
	}
	return out
}

func TestJoinRetryAfterAllowsTheAttemptOnceTheWaitPasses(t *testing.T) {
	now := time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC)
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 40).Draw(t, "n")
		attempts := make([]time.Time, n)
		for i := range attempts {
			attempts[i] = now.Add(-time.Duration(rapid.IntRange(0, 4000).Draw(t, "age_sec")) * time.Second)
		}
		retry, limited := JoinRetryAfter(attempts, now)
		if !limited {
			return
		}
		if retry <= 0 {
			t.Fatalf("limited with retry %d", retry)
		}
		later := now.Add(time.Duration(retry) * time.Second)
		if _, still := JoinRetryAfter(attempts, later); still {
			t.Fatalf("still limited after waiting %d s", retry)
		}
	})
}

func TestValidateJoin(t *testing.T) {
	ok := JoinInput{Code: "042137", DisplayName: "  Ерлан ", Device: Device{DeviceID: "device-123", Platform: PlatformAndroid}}
	got, err := ValidateJoin(ok)
	require.NoError(t, err)
	require.Equal(t, "Ерлан", got.DisplayName)

	cases := []struct {
		name   string
		mutate func(*JoinInput)
		field  string
		reason string
	}{
		{"three digit code is invalid", func(in *JoinInput) { in.Code = "123" }, "code", ReasonInvalid},
		{"four digit code of the old contract is invalid", func(in *JoinInput) { in.Code = "0421" }, "code", ReasonInvalid},
		{"seven digit code is invalid", func(in *JoinInput) { in.Code = "0421378" }, "code", ReasonInvalid},
		{"letters in code are invalid", func(in *JoinInput) { in.Code = "12a456" }, "code", ReasonInvalid},
		{"empty name is required", func(in *JoinInput) { in.DisplayName = "   " }, "display_name", ReasonRequired},
		{"name over 40 runes is too long", func(in *JoinInput) { in.DisplayName = repeat("я", 41) }, "display_name", ReasonTooLong},
		{"short device id is too short", func(in *JoinInput) { in.DeviceID = "abc" }, "device_id", ReasonTooShort},
		{"web platform is invalid", func(in *JoinInput) { in.Platform = "web" }, "platform", ReasonInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := ok
			c.mutate(&in)
			_, err := ValidateJoin(in)
			var v *ValidationError
			require.ErrorAs(t, err, &v)
			require.Equal(t, c.reason, v.Fields[c.field])
		})
	}
}

func TestValidateRegister(t *testing.T) {
	in := RegisterInput{Email: "a@b.kz", Password: "12345678", DisplayName: "Арсен", VenueName: "Абая", Device: Device{DeviceID: "device-123", Platform: PlatformIOS}}
	_, err := ValidateRegister(in)
	require.NoError(t, err)

	in.Password = "1234567"
	in.Email = "nobody"
	_, err = ValidateRegister(in)
	var v *ValidationError
	require.ErrorAs(t, err, &v)
	require.Equal(t, map[string]string{"password": ReasonTooShort, "email": ReasonInvalid}, v.Fields)

	in.Password = repeat("x", 73)
	in.Email = "a@b.kz"
	_, err = ValidateRegister(in)
	require.ErrorAs(t, err, &v)
	require.Equal(t, ReasonTooLong, v.Fields["password"])
}

func TestValidatePushToken(t *testing.T) {
	require.NoError(t, ValidatePushToken("ExponentPushToken[xxxxxxxxxxxxxxxxxxxxxx]"))
	require.NoError(t, ValidatePushToken("ExpoPushToken[yyyy]"))
	require.ErrorIs(t, ValidatePushToken("fcm:abc"), ErrValidation)
	require.ErrorIs(t, ValidatePushToken("ExponentPushToken[abc"), ErrValidation)
	require.ErrorIs(t, ValidatePushToken("ExponentPushToken["+repeat("a", 200)+"]"), ErrValidation)
}

func TestFormatJoinCode(t *testing.T) {
	require.Equal(t, "000007", FormatJoinCode(7))
	require.Equal(t, "999999", FormatJoinCode(999999))
	require.Equal(t, "000000", FormatJoinCode(JoinCodeSpace))
}

func TestFormatJoinCodeAlwaysValidates(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, JoinCodeSpace-1).Draw(t, "n")
		code := FormatJoinCode(n)
		in := JoinInput{Code: code, DisplayName: "Ерлан", Device: Device{DeviceID: "device-123", Platform: PlatformAndroid}}
		if _, err := ValidateJoin(in); err != nil {
			t.Fatalf("generated code %q fails validation: %v", code, err)
		}
	})
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
