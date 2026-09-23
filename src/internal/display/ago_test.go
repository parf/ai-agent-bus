package display

import (
	"testing"
	"time"
)

func TestAgoIsCoarse(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		back time.Duration
		want string
	}{
		{-time.Minute, "just now"}, // a clock a little ahead is still now
		{30 * time.Second, "just now"},
		{3 * time.Minute, "3m ago"},
		{5 * time.Hour, "5h ago"},
		{72 * time.Hour, "3d ago"},
	} {
		if got := Ago(now.Add(-c.back), now); got != c.want {
			t.Errorf("Ago(now-%v) = %q, want %q", c.back, got, c.want)
		}
	}
}
