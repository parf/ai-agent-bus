package main

import (
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// `agent-bus run` states the machine's host on every read: the real command, a
// real daemon handler, and the name's listing shows the host once the agent is
// reading. A run that dropped the host would leave the list empty.
func TestAgentBusRunStatesItsHostOnItsReads(t *testing.T) {
	want, why := api.ReaderHost()
	if want == "" {
		t.Skipf("this machine's hostname states no host: %s", why)
	}
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
	script := filepath.Join(t.TempDir(), "agent.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho ok\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestFlagCLIHelper$", "--", "run", "#hosted@h", script, "--sandbox=off")
	env := []string{}
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "AGENT_BUS_") && !strings.HasPrefix(v, "XDG_") {
			env = append(env, v)
		}
	}
	cmd.Env = append(env, "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0", "AGENT_BUS_ADDR="+server.URL, "AGENT_BUS_TOKEN="+token,
		"XDG_STATE_HOME="+t.TempDir(), "XDG_CACHE_HOME="+t.TempDir())
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	for end := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if r, ok := b.Lookup("owner@h", "#hosted@h"); ok && len(r.Hosts) == 1 {
			if r.Hosts[0].Name != want {
				t.Fatalf("the run stated host %q, want %q", r.Hosts[0].Name, want)
			}
			return
		}
		if time.Now().After(end) {
			t.Fatal("`agent-bus run` never stated its host on a read")
		}
	}
}
