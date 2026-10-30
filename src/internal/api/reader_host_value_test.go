package api

import (
	"strings"
	"testing"
)

// The hostname a reader states is AGENT_BUS_HOST when set; one the daemon would
// refuse is not sent, and the reason names where it came from.
func TestReaderHostFollowsTheEnvironmentAndRefusesWhatTheDaemonWould(t *testing.T) {
	t.Setenv(HostEnv, "srv-1.example")
	if host, why := ReaderHost(); host != "srv-1.example" || why != "" {
		t.Fatalf("an environment host: %q %q", host, why)
	}
	t.Setenv(HostEnv, "  srv2  ")
	if host, _ := ReaderHost(); host != "srv2" {
		t.Fatalf("a padded host: %q, want it trimmed", host)
	}
	t.Setenv(HostEnv, "bad_host")
	host, why := ReaderHost()
	if host != "" || !strings.Contains(why, HostEnv) || !strings.Contains(why, "reads state no host") {
		t.Fatalf("a bad host: %q %q, want none and a reason naming %s", host, why, HostEnv)
	}
	t.Setenv(HostEnv, "")
	if host, why := ReaderHost(); why != "" && host != "" {
		t.Fatalf("the OS hostname: %q %q", host, why)
	}
}
