package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// RTA1c: an empty --tag (or --tagged) value is a mistake, not "no filter".
// The real CLI against a real daemon handler: ls and call --tagged refuse it
// by name and exit non-zero, a refused call reaches no agent (the inboxes are
// COUNTED), and #bus ls answers the same refusal instead of a listing.
func TestAnEmptyTagValueIsRefusedNotDropped(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	daemon := api.New(b, tokens, "owner@h")
	defer daemon.Close()
	if err := b.EstablishDaemonAgent("h"); err != nil {
		t.Fatal(err)
	}
	defer daemon.ServeDaemonAgent(context.Background())()
	server := httptest.NewServer(daemon.Handler())
	defer server.Close()
	token, err := tokens.Issue("owner@h")
	if err != nil {
		t.Fatal(err)
	}
	agents := map[string][]string{"#only-a@h": {"alpha"}, "#only-b@h": {"beta"}, "#both@h": {"alpha", "beta"}}
	for name, tags := range agents {
		if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "owner@h", Tags: tags}); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) (string, int) {
		t.Helper()
		deadline, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		cmd := exec.CommandContext(deadline, os.Args[0], append([]string{"-test.run=^TestFlagCLIHelper$", "--"}, args...)...)
		env := []string{}
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "AGENT_BUS_") {
				env = append(env, v)
			}
		}
		cmd.Env = append(env, "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0", "AGENT_BUS_ADDR="+server.URL, "AGENT_BUS_TOKEN="+token, "XDG_CACHE_HOME="+t.TempDir())
		out, _ := cmd.CombinedOutput()
		return string(out), cmd.ProcessState.ExitCode()
	}
	refused := func(option string, args ...string) {
		t.Helper()
		out, code := run(args...)
		if code == 0 || !strings.Contains(out, option+" wants a tag") {
			t.Errorf("%v: exit %d, want a refusal naming %s:\n%s", args, code, option, out)
		}
		for name := range agents {
			if strings.Contains(out, name) {
				t.Errorf("%v listed or reached %s instead of refusing:\n%s", args, name, out)
			}
		}
	}
	refused("--tag", "ls", "--kind=agent", "--tag=alpha", "--tag=")
	refused("--tag", "ls", "--kind=agent", "--tag=", "--tag=alpha")
	refused("--tag", "ls", "--kind=agent", "--tag=")
	refused("--tag", "ls", "--kind=agent", "--tag", "")
	refused("--tagged", "call", "--tagged=alpha", "--tagged=", "--wait=300ms", "x")
	refused("--tagged", "call", "--tagged=", "--wait=300ms", "x")
	for name := range agents {
		if r, _ := b.Lookup("owner@h", name); r.Queued != 0 {
			t.Errorf("a refused call left %d message(s) for %s", r.Queued, name)
		}
	}
	// A trailing comma is a list with one tag, as before.
	out, code := run("ls", "--kind=agent", "--tag=alpha,")
	var recs []protocol.Record
	if err := json.Unmarshal([]byte(out), &recs); code != 0 || err != nil {
		t.Fatalf("ls --tag=alpha, : exit %d, %v\n%s", code, err, out)
	}
	if n := strings.Count(out, `"name":"#`); n != 2 {
		t.Errorf("ls --tag=alpha, listed %d records, want its 2:\n%s", n, out)
	}
	// #bus ls answers the refusal as its body; the call itself is answered.
	for _, body := range []string{"ls --tag=alpha --tag=", "ls --tag=", "ls --tag= --tag=alpha"} {
		out, _ := run("call", core.DaemonAgent, "--wait=5s", body)
		if !strings.Contains(out, "error: --tag wants a tag") {
			t.Errorf("#bus %q: want the refusal, got:\n%s", body, out)
		}
		for name := range agents {
			if strings.Contains(out, name) {
				t.Errorf("#bus %q listed %s instead of refusing:\n%s", body, name, out)
			}
		}
	}
}
