package domain

import "time"

// DBTime normalizes a timestamp to what Postgres timestamptz stores, so a value
// written and read back compares equal to the original.
func DBTime(t time.Time) time.Time {
	return t.UTC().Truncate(time.Microsecond)
}

func secondsAfter(t time.Time, sec int) time.Time {
	return t.Add(time.Duration(sec) * time.Second)
}
