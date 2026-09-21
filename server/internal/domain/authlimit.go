package domain

import "time"

// Owner sign-in limits, tech.md section 19.16.
const (
	LoginIPPerMinute     = 10
	LoginEmailFailsPer15 = 5
	loginEmailWindow     = 15 * time.Minute
	RegisterIPPerHour    = 5
)

// LoginAttempts is what the login limits look at. IP holds every attempt from
// the address, EmailFailed the wrong passwords for the address tried. Order
// does not matter.
type LoginAttempts struct {
	IP          []time.Time
	EmailFailed []time.Time
}

// LoginDecision returns the longest wait of the exceeded login limits.
func LoginDecision(a LoginAttempts, now time.Time) (retryAfterSec int, limited bool) {
	retry := max(
		windowRetryAfter(a.IP, now, time.Minute, LoginIPPerMinute),
		windowRetryAfter(a.EmailFailed, now, loginEmailWindow, LoginEmailFailsPer15),
	)
	return retry, retry > 0
}

// RegisterDecision limits account creation per address.
func RegisterDecision(ip []time.Time, now time.Time) (retryAfterSec int, limited bool) {
	retry := windowRetryAfter(ip, now, time.Hour, RegisterIPPerHour)
	return retry, retry > 0
}
