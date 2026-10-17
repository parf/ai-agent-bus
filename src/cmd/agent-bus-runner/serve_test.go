package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A later list wins a name an earlier one set, in place: the agent's PATH
// beats the runner's, and the runner's stamp of the sender beats an env file.
func TestTheChildsEnvironmentLetsTheLaterLayerWin(t *testing.T) {
	got := mergeEnv(
		[]string{"PATH=/usr/bin", "HOME=/h"},
		[]string{"PATH=/opt/agent/bin", "AGENT_BUS_FROM=spoof"},
		[]string{"AGENT_BUS_FROM=alice@h"},
	)
	if strings.Join(got, " ") != "PATH=/opt/agent/bin HOME=/h AGENT_BUS_FROM=alice@h" {
		t.Fatalf("merged %q", got)
	}
}

// Two installed agents that come to one name are both refused, by name,
// rather than served as one with the other's command.
func TestTwoAgentsWithOneNameAreBothRefused(t *testing.T) {
	// A daemon that refuses everything: a start that got past the check
	// would be refused for another reason, and the assertion below says so.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "fixture: no daemon here", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := &client{http: srv.Client(), base: srv.URL}
	r := &runner{realm: "h", served: map[string]*served{}, say: io.Discard, bus: c, owner: c}
	r.installed = []installed{
		{Key: "a", Dir: "a", Agent: agentFile{Command: "x"}, Host: hostEntry{Name: "#same@h"}},
		{Key: "b", Dir: "b", Agent: agentFile{Command: "y"}, Host: hostEntry{Name: "#same@h"}},
	}
	r.serve(context.Background())
	for _, in := range r.installed {
		if !strings.Contains(in.Refused, "#same@h is also the name of") {
			t.Errorf("%s: refused %q", in.Key, in.Refused)
		}
	}
	if len(r.served) != 0 {
		t.Fatalf("served %d", len(r.served))
	}
}
