package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

// `ls --table <name>` itself prints the Hosts line after the table, as the
// real command does: a formatter nobody calls would pass the test above.
func TestLsTableOnOneNamePrintsTheHostsLine(t *testing.T) {
	zero := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(protocol.Record{Name: "#pool@h", Kind: "agent", Readers: &zero,
			Hosts: []protocol.ReaderHost{{Name: "srv1", LastRead: time.Now()}}})
	}))
	defer server.Close()
	oldTransport, oldStdout := transport, os.Stdout
	defer func() { transport, os.Stdout = oldTransport, oldStdout }()
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	t.Setenv("AGENT_BUS_TOKEN", "fixture")
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	os.Stdout = write
	err = ls([]string{"--table", "#pool@h"})
	write.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "Hosts  srv1 (now)") {
		t.Fatalf("ls --table on one name omitted the Hosts line:\n%s", out)
	}
	if !strings.Contains(string(out), "#pool@h") {
		t.Fatalf("the table is gone:\n%s", out)
	}
}
