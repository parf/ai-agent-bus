package main

import (
	"context"
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

// The real CLI against a real daemon handler: `consume --host=<name>` is listed
// under the name's hosts, and a value that is not a hostname is refused naming
// the host, so the mistake is not silently dropped.
func TestConsumeHostIsStatedAndRefusedByName(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	daemon := api.New(b, tokens, "owner@h")
	defer daemon.Close()
	server := httptest.NewServer(daemon.Handler())
	defer server.Close()
	token, err := tokens.Issue("owner@h")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "#pool@h", Kind: protocol.KindAgent, Owner: "owner@h"}); err != nil {
		t.Fatal(err)
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
	if out, code := run("consume", "--inbox=#pool@h", "--wait=100ms", "--host=bad_host"); code == 0 || !strings.Contains(out, "host") {
		t.Fatalf("a bad host: exit %d, %q, want a refusal naming the host", code, out)
	}
	if r, _ := b.Lookup("owner@h", "#pool@h"); len(r.Hosts) != 0 {
		t.Fatalf("a refused host was listed: %+v", r.Hosts)
	}
	run("consume", "--inbox=#pool@h", "--wait=100ms", "--host=srv1")
	r, _ := b.Lookup("owner@h", "#pool@h")
	if len(r.Hosts) != 1 || r.Hosts[0].Name != "srv1" {
		t.Fatalf("hosts %+v, want srv1", r.Hosts)
	}
}
