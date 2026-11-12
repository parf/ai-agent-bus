package schedule

import (
	"testing"
	"time"
)

func TestOneShotTimeForms(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 7, 30, 0, time.UTC)
	for text, want := range map[string]string{"90s": "10:09:00", "+15m": "10:22:30", "2h": "12:07:30", "3d": "2026-11-15T10:07:30Z"} {
		got, err := In(text, now)
		s := got.Format("15:04:05")
		if len(want) > 8 {
			s = got.Format(time.RFC3339)
		}
		if err != nil || s != want {
			t.Errorf("in %s: %s %v, want %s", text, s, err, want)
		}
	}
	for _, bad := range []string{"0s", "1h30m", "90", "-5m", "10w", "12345678s"} {
		if _, err := In(bad, now); err == nil {
			t.Errorf("in %s accepted", bad)
		}
	}
	if _, err := In("367d", now); err == nil || err.Error() != "at most 366 days ahead" {
		t.Errorf("in 367d: %v", err)
	}
	for text, want := range map[string]string{
		"10:08": "2026-11-12T10:08:00Z", "10:07": "2026-11-13T10:07:00Z", "09:00": "2026-11-13T09:00:00Z",
		"2026-11-12T05:30:00-05:00": "2026-11-12T10:30:00Z", "2026-11-12T10:07:10Z": "2026-11-12T10:07:30Z",
	} {
		got, err := At(text, now)
		if err != nil || got.Format(time.RFC3339) != want {
			t.Errorf("at %s: %s %v, want %s", text, got.Format(time.RFC3339), err, want)
		}
	}
	for text, why := range map[string]string{
		"9:30":                 "a time of day is HH:MM, two digits each, 00:00 to 23:59",
		"noon":                 "at takes HH:MM or an RFC 3339 instant with an offset, such as 09:30 or 2026-10-29T09:30:00Z",
		"2026-11-12T10:00:00":  "at is an RFC 3339 instant with an offset, such as Z or -04:00",
		"2026-11-12T10:06:59Z": "at is in the past",
		"2027-11-14T10:00:00Z": "at most 366 days ahead",
	} {
		if _, err := At(text, now); err == nil || err.Error() != why {
			t.Errorf("at %s: %v, want %q", text, err, why)
		}
	}
}
