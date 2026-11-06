package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// J104. The owner: "bus record does not mean it running". A listing with no --kind and no --all shows
// only the agents being read now; adding --tag narrows that listing by tag and does not widen it to
// records nobody reads (mdhouse's start-agents.sh took such records for live sessions). --all is how
// to see every record. The real CLI against a real daemon handler, a private bus; the names are COUNTED.
func TestLsTagKeepsTheReadersRuleUnlessAll(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens())
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
	for name, tags := range map[string][]string{
		"#live@h": {"grp"}, "#dead@h": {"grp"}, "#dead2@h": {"grp"}, "#other@h": {"elsewhere"}, "#deadother@h": {"elsewhere"},
	} {
		if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "owner@h", Tags: tags}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	for _, name := range []string{"#live@h", "#other@h"} {
		go func() { _, _ = b.Consume(ctx, name, "", "", false, false) }() // an outstanding read: a reader
	}
	run := func(args ...string) string {
		t.Helper()
		deadline, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(deadline, os.Args[0], append([]string{"-test.run=^TestFlagCLIHelper$", "--"}, args...)...)
		env := []string{}
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "AGENT_BUS_") {
				env = append(env, v)
			}
		}
		cmd.Env = append(env, "AB_CLI_FLAG_HELPER=1", "GORACE=atexit_sleep_ms=0", "AGENT_BUS_ADDR="+server.URL, "AGENT_BUS_TOKEN="+token, "XDG_CACHE_HOME="+t.TempDir())
		out, _ := cmd.CombinedOutput()
		return string(out)
	}
	listed := func(args ...string) []string {
		t.Helper()
		out := run(append([]string{"ls"}, args...)...)
		var recs []protocol.Record
		if err := json.Unmarshal([]byte(out), &recs); err != nil {
			t.Fatalf("ls %v: %v\n%s", args, err, out)
		}
		var names []string
		for _, r := range recs {
			if r.Kind == protocol.KindAgent && strings.HasSuffix(r.Name, "@h") {
				names = append(names, r.Name)
			}
		}
		slices.Sort(names)
		return names
	}
	// The readers register in goroutines: wait until both are counted.
	for deadline := time.Now().Add(5 * time.Second); len(listed()) < 2 && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
	}
	want := func(args []string, names ...string) {
		t.Helper()
		if got := listed(args...); !slices.Equal(got, names) {
			t.Errorf("ls %v listed %v, want %v", args, got, names)
		}
	}
	want(nil, "#live@h", "#other@h")
	want([]string{"--tag=grp"}, "#live@h")
	want([]string{"--tag=elsewhere"}, "#other@h")
	want([]string{"--tag=grp", "--all"}, "#dead2@h", "#dead@h", "#live@h")
	want([]string{"--tag=grp", "--kind=agent"}, "#dead2@h", "#dead@h", "#live@h")
	want([]string{"--all"}, "#dead2@h", "#dead@h", "#deadother@h", "#live@h", "#other@h")
}
