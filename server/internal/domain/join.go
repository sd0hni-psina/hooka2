package domain

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Join rate limit windows, tech.md sections 7 and 19.4.
const (
	JoinPerMinute = 5
	JoinPerHour   = 20
)

// JoinRetryAfter decides whether a device may try another code. attempts are
// the recorded attempts of the device, in any order. It returns the seconds
// until the oldest attempt of an exceeded window leaves it.
func JoinRetryAfter(attempts []time.Time, now time.Time) (retryAfterSec int, limited bool) {
	check := func(window time.Duration, limit int) int {
		var inWindow []time.Time
		for _, a := range attempts {
			if now.Sub(a) < window {
				inWindow = append(inWindow, a)
			}
		}
		if len(inWindow) < limit {
			return 0
		}
		// The window frees a slot when enough attempts age out that fewer
		// than limit remain: that is the (len-limit+1)-th oldest.
		oldest := nthOldest(inWindow, len(inWindow)-limit)
		wait := oldest.Add(window).Sub(now)
		return int(math.Ceil(wait.Seconds()))
	}
	m := check(time.Minute, JoinPerMinute)
	h := check(time.Hour, JoinPerHour)
	retry := max(m, h)
	return retry, retry > 0
}

func nthOldest(ts []time.Time, n int) time.Time {
	sorted := slices.Clone(ts)
	slices.SortFunc(sorted, time.Time.Compare)
	return sorted[n]
}

// FormatJoinCode renders 0..9999 as four digits.
func FormatJoinCode(n int) string {
	return fmt.Sprintf("%04d", n%10000)
}

func checkLen(v *ValidationError, field, value string, minLen, maxLen int) {
	n := utf8.RuneCountInString(value)
	switch {
	case n == 0:
		v.Add(field, ReasonRequired)
	case n < minLen:
		v.Add(field, ReasonTooShort)
	case n > maxLen:
		v.Add(field, ReasonTooLong)
	}
}

func checkPlatform(v *ValidationError, p Platform) {
	if p == "" {
		v.Add("platform", ReasonRequired)
	} else if !p.Valid() {
		v.Add("platform", ReasonInvalid)
	}
}

// Device identifies the phone that holds a session.
type Device struct {
	DeviceID string
	Platform Platform
}

func (d Device) check(v *ValidationError) {
	checkLen(v, "device_id", d.DeviceID, 8, 128)
	checkPlatform(v, d.Platform)
}

type JoinInput struct {
	Code        string
	DisplayName string
	Device
}

// ValidateJoin trims and checks the input of POST /join, tech.md section 19.1.
func ValidateJoin(in JoinInput) (JoinInput, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.DeviceID = strings.TrimSpace(in.DeviceID)
	var v ValidationError
	if in.Code == "" {
		v.Add("code", ReasonRequired)
	} else if !isDigits(in.Code, 4) {
		v.Add("code", ReasonInvalid)
	}
	checkLen(&v, "display_name", in.DisplayName, 1, 40)
	in.check(&v)
	return in, v.Err()
}

type RegisterInput struct {
	Email       string
	Password    string
	DisplayName string
	VenueName   string
	Device
}

func ValidateRegister(in RegisterInput) (RegisterInput, error) {
	in.Email = strings.TrimSpace(in.Email)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.VenueName = strings.TrimSpace(in.VenueName)
	in.DeviceID = strings.TrimSpace(in.DeviceID)
	var v ValidationError
	checkEmail(&v, in.Email)
	checkPassword(&v, in.Password)
	checkLen(&v, "display_name", in.DisplayName, 1, 40)
	checkLen(&v, "venue_name", in.VenueName, 1, 80)
	in.check(&v)
	return in, v.Err()
}

type LoginInput struct {
	Email    string
	Password string
	Device
}

func ValidateLogin(in LoginInput) (LoginInput, error) {
	in.Email = strings.TrimSpace(in.Email)
	in.DeviceID = strings.TrimSpace(in.DeviceID)
	var v ValidationError
	checkEmail(&v, in.Email)
	if in.Password == "" {
		v.Add("password", ReasonRequired)
	}
	in.check(&v)
	return in, v.Err()
}

func checkEmail(v *ValidationError, email string) {
	checkLen(v, "email", email, 3, 254)
	if email != "" && !strings.Contains(email, "@") {
		v.Add("email", ReasonInvalid)
	}
}

// Passwords are measured in bytes: bcrypt ignores everything past 72.
func checkPassword(v *ValidationError, p string) {
	switch {
	case p == "":
		v.Add("password", ReasonRequired)
	case len(p) < 8:
		v.Add("password", ReasonTooShort)
	case len(p) > 72:
		v.Add("password", ReasonTooLong)
	}
}

// ValidatePushToken checks the Expo token format of tech.md section 19.4.
func ValidatePushToken(token string) error {
	var v ValidationError
	token = strings.TrimSpace(token)
	ok := len(token) <= 200 && strings.HasSuffix(token, "]") &&
		(strings.HasPrefix(token, "ExponentPushToken[") || strings.HasPrefix(token, "ExpoPushToken["))
	if !ok {
		v.Add("expo_push_token", ReasonInvalid)
	}
	return v.Err()
}

func isDigits(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
