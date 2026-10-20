package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestAgentWideHostEnvSatisfiesRequiredKeysWithoutAnInstance(t *testing.T) {
	r, b, _ := configurationFixture(t, "", `if [ "$REQUIRED_KEY" = fixture-private ]; then printf configured; else printf missing; fi`)
	r.mu.Lock()
	s := r.served["#fixture@h"]
	r.mu.Unlock()
	writeFile(t, filepath.Join(r.agents, "fixture", "env.dist"), "REQUIRED_KEY= # required\n")
	// The logical runner/ prefix names --host itself, not a nested runner directory.
	writeFile(t, filepath.Join(r.host, "runner", "fixture", "env"), "REQUIRED_KEY=wrong-location\n")
	if _, _, why := r.environmentWithSecrets(s.in, s.bus); !strings.Contains(why, "needs REQUIRED_KEY") {
		t.Fatalf("missing required key not refused: %q", why)
	}
	writeFile(t, filepath.Join(r.host, "fixture", "env"), "REQUIRED_KEY=fixture-private\n")
	env, private, why := r.environmentWithSecrets(s.in, s.bus)
	if why != "" || !strings.Contains(strings.Join(env, "\n"), "REQUIRED_KEY=fixture-private") || !strings.Contains(strings.Join(private, "\n"), "REQUIRED_KEY=fixture-private") {
		t.Fatal("plain host env did not satisfy required key with private provenance")
	}
	r.reload()
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#fixture@h", Body: "probe"}); err != nil {
		t.Fatal(err)
	}
	if got := configurationReply(t, b).Body; got != "configured" {
		t.Fatalf("plain agent reply=%q", got)
	}
}
