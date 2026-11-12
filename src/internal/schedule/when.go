package schedule

import (
	"regexp"
	"strconv"
	"time"
)

// The one-shot time forms of #cron's later and #at, read in UTC.
const (
	Skew    = 30 * time.Second     // an instant this far in the past still fires now
	Horizon = 366 * 24 * time.Hour // nothing further ahead
	MaxBody = 16 * 1024            // a message's bytes
)

var (
	delay    = regexp.MustCompile(`^\+?([1-9][0-9]{0,6})([smhd])$`)
	clock    = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)
	clockish = regexp.MustCompile(`^[0-9]{1,2}:[0-9]{1,2}(:[0-9]{2})?$`)
	digit    = regexp.MustCompile(`[0-9]`)
	units    = map[string]time.Duration{"s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour}
)

// In is now plus a delay: 90s, 30m, 2h or 3d, an optional leading +, one unit.
func In(text string, now time.Time) (time.Time, error) {
	m := delay.FindStringSubmatch(text)
	if m == nil {
		return time.Time{}, Error("in is <n>s, <n>m, <n>h or <n>d, such as 90s or 30m; one unit, n at least 1")
	}
	n, _ := strconv.Atoi(m[1])
	return within(now.Add(time.Duration(n)*units[m[2]]), now)
}

// At is HH:MM, the first such UTC time strictly after the current minute, or
// an RFC 3339 instant with an offset.
func At(text string, now time.Time) (time.Time, error) {
	now = now.UTC()
	if m := clock.FindStringSubmatch(text); m != nil {
		h, _ := strconv.Atoi(m[1])
		mi, _ := strconv.Atoi(m[2])
		after := now.Truncate(time.Minute).Add(time.Minute)
		t := time.Date(now.Year(), now.Month(), now.Day(), h, mi, 0, 0, time.UTC)
		if t.Before(after) {
			t = t.AddDate(0, 0, 1)
		}
		return t, nil
	}
	if clockish.MatchString(text) {
		return time.Time{}, Error("a time of day is HH:MM, two digits each, 00:00 to 23:59")
	}
	if !digit.MatchString(text) {
		return time.Time{}, Error("at takes HH:MM or an RFC 3339 instant with an offset, such as 09:30 or 2026-10-29T09:30:00Z")
	}
	t, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}, Error("at is an RFC 3339 instant with an offset, such as Z or -04:00")
	}
	if t.Before(now.Add(-Skew)) {
		return time.Time{}, Error("at is in the past")
	}
	return within(t, now)
}

func within(t, now time.Time) (time.Time, error) {
	if t.After(now.Add(Horizon)) {
		return time.Time{}, Error("at most 366 days ahead")
	}
	if t.Before(now) {
		t = now
	}
	return t.UTC().Truncate(time.Second), nil
}
