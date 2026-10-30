package main

import (
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// One record's table is followed by a Hosts line, each host with how long ago
// it last read; a name no host has read has none, and the table is the same.
func TestHostsLineShowsEachHostAndHowLongAgo(t *testing.T) {
	now := time.Now()
	r := protocol.Record{Name: "#pool@h", Hosts: []protocol.ReaderHost{
		{Name: "srv1", LastRead: now},
		{Name: "srv2", LastRead: now.Add(-40 * time.Second)},
		{Name: "srv3", LastRead: now.Add(-100 * time.Second)},
	}}
	if got, want := hostsLine(r, now), "Hosts  srv1 (now), srv2 (40s), srv3 (1m)"; got != want {
		t.Fatalf("hosts line %q, want %q", got, want)
	}
	r.HostsCut = true
	if got := hostsLine(r, now); !strings.Contains(got, "oldest were dropped") {
		t.Fatalf("a cut list says nothing of it: %q", got)
	}
	if got := hostsLine(protocol.Record{Name: "#pool@h"}, now); got != "" {
		t.Fatalf("a name with no hosts has a line %q", got)
	}
}
