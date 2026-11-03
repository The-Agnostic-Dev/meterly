// Package clock keeps every date in the estate's local time (Africa/Lagos).
package clock

import (
	"fmt"
	"time"
	_ "time/tzdata" // the container image has no zoneinfo
)

// Lagos is the estate's time zone.
var Lagos = mustLoad("Africa/Lagos")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("clock: load %s: %v", name, err))
	}
	return loc
}

// Now returns the current time in Lagos.
func Now() time.Time { return time.Now().In(Lagos) }

// Today returns midnight today in Lagos.
func Today() time.Time { return StartOfDay(Now()) }

// StartOfDay returns midnight of t's calendar day in Lagos.
func StartOfDay(t time.Time) time.Time {
	y, m, d := t.In(Lagos).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, Lagos)
}

// StartOfMonth returns midnight on the first day of t's month in Lagos.
func StartOfMonth(t time.Time) time.Time {
	y, m, _ := t.In(Lagos).Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, Lagos)
}

// MonthKey formats t as "2006-01".
func MonthKey(t time.Time) string { return t.In(Lagos).Format("2006-01") }

// ParseMonth parses a "2006-01" key into the first day of that month.
func ParseMonth(key string) (time.Time, error) {
	return time.ParseInLocation("2006-01", key, Lagos)
}

// ParseDate parses a "2006-01-02" form value as a Lagos calendar day.
func ParseDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, Lagos)
}

// MonthLabel turns "2026-10" into "October 2026". Unknown keys are returned as given.
func MonthLabel(key string) string {
	t, err := ParseMonth(key)
	if err != nil {
		return key
	}
	return t.Format("January 2006")
}

// RecentMonths returns the keys of the current month and the n-1 months before it, newest first.
func RecentMonths(n int) []string {
	start := StartOfMonth(Now())
	keys := make([]string, 0, n)
	for i := 0; i < n; i++ {
		keys = append(keys, MonthKey(start.AddDate(0, -i, 0)))
	}
	return keys
}
