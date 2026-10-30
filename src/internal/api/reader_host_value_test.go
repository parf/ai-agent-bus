package api

import (
	"errors"
	"strings"
	"testing"
)

// The hostname a reader states is the machine's own; one the daemon would
// refuse is not sent, so the read still goes, and the reason names the hostname.
func TestReaderHostIsTheMachinesAndNeverFailsTheRead(t *testing.T) {
	old := hostname
	defer func() { hostname = old }()
	for _, tc := range []struct {
		name, host string
		err        error
		want       string
		why        bool
	}{
		{"valid", "srv-1.example", nil, "srv-1.example", false},
		{"underscore", "bad_host", nil, "", true},
		{"too long", strings.Repeat("a", 65), nil, "", true},
		{"empty", "", nil, "", false},
		{"unavailable", "srv", errors.New("no hostname"), "", false},
	} {
		hostname = func() (string, error) { return tc.host, tc.err }
		host, why := ReaderHost()
		if host != tc.want || (why != "") != tc.why {
			t.Errorf("%s: host %q why %q, want host %q and a reason %v", tc.name, host, why, tc.want, tc.why)
		}
	}
}

// A unit's AGENT_BUS_READER_HOST wins over the machine's hostname, and is
// held to the same rule: one the daemon would refuse is not sent.
func TestReaderHostOverrideWinsAndIsChecked(t *testing.T) {
	old := hostname
	defer func() { hostname = old }()
	hostname = func() (string, error) { return "machine", nil }
	for _, tc := range []struct {
		name, env, want string
		why             bool
	}{
		{"override", "edge-2.example", "edge-2.example", false},
		{"unset", "", "machine", false},
		{"bad override", "bad_host", "", true},
		{"too long", strings.Repeat("a", 65), "", true},
	} {
		t.Setenv("AGENT_BUS_READER_HOST", tc.env)
		host, why := ReaderHost()
		if host != tc.want || (why != "") != tc.why {
			t.Errorf("%s: host %q why %q, want host %q and a reason %v", tc.name, host, why, tc.want, tc.why)
		}
	}
}
