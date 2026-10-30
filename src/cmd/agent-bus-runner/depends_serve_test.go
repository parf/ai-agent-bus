package main

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// The runner itself, not only load(), brings an agent up after the ones it
// depends on: a re-sort anywhere between load and the start loop would put
// them back in key order.
func TestTheRunnerAdoptsAgentsInDependencyOrder(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "owner@h")
	if err != nil {
		t.Fatal(err)
	}
	server := api.New(b, tokens, "owner@h")
	t.Cleanup(server.Close)
	names := []string{"#runner@h", "#a@h", "#m@h", "#z@h"}
	for _, name := range names {
		if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "owner@h"}); err != nil {
			t.Fatal(err)
		}
	}
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(httpServer.Close)
	ctx, cancel := context.WithCancel(context.Background())
	root := t.TempDir()
	journal := &secretJournal{}
	r := &runner{ctx: ctx, name: "#runner@h", person: "owner@h", realm: "h", host: filepath.Join(root, "runner"), agents: filepath.Join(root, "agent.d"), say: journal, served: make(map[string]*served)}
	tok, err := tokens.Issue(r.name)
	if err != nil {
		t.Fatal(err)
	}
	r.bus = &client{http: httpServer.Client(), base: httpServer.URL, token: tok}
	r.addr = httpServer.URL
	if err := os.MkdirAll(filepath.Join(r.host, "tokens"), 0o700); err != nil {
		t.Fatal(err)
	}
	host := map[string]any{}
	// a needs z, m needs a: key order is a, m, z; the start order must be z, a, m.
	for key, depends := range map[string][]string{"a": {"#z"}, "m": {"#a"}, "z": nil} {
		dir := filepath.Join(r.agents, key)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		manifest := map[string]any{"description": "Test agent", "command": "/bin/true", "serves": "runner", "processes": 1}
		if depends != nil {
			manifest["depends"] = depends
		}
		if err := os.WriteFile(filepath.Join(dir, "agent.json"), mustJSON(manifest), 0o600); err != nil {
			t.Fatal(err)
		}
		host[key] = map[string]any{"sandbox": "off"}
		tok, err := tokens.Issue("#" + key + "@h")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(r.tokenFile("#"+key+"@h"), []byte(tok), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(r.host, "agents.json"), mustJSON(host), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); r.stopKept() })
	r.reload()
	var got []string
	for _, m := range regexp.MustCompile(`serving (#\w+)@h`).FindAllStringSubmatch(journal.String(), -1) {
		got = append(got, m[1])
	}
	if want := []string{"#z", "#a", "#m"}; len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("adopted %v, want %v\n%s", got, want, journal.String())
	}
}
