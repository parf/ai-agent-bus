package main

import (
	"testing"
	"time"
)

// Time left reads as the web face shows it, and never goes negative.
func TestLeftText(t *testing.T) {
	for d, want := range map[time.Duration]string{
		-3 * time.Second: "0s", 30 * time.Second: "30s", 4*time.Minute + 10*time.Second: "4m",
		time.Hour: "1h", time.Hour + 5*time.Minute: "1h 5m", 24 * time.Hour: "24h",
	} {
		if got := leftText(d); got != want {
			t.Errorf("leftText(%v) = %q, want %q", d, got, want)
		}
	}
}
