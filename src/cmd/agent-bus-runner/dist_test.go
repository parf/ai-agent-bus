package main

import (
	"path/filepath"
	"testing"
)

// The documented samples hold every key, and each must load as the runner
// reads it: strictly, so a key renamed in code fails here before it misleads.
func TestTheDocumentedSamplesLoad(t *testing.T) {
	docs := filepath.Join("..", "..", "..", "docs", "runner")
	var a agentFile
	if err := strict(filepath.Join(docs, "agent.json.dist"), &a); err != nil {
		t.Fatal(err)
	}
	if _, err := a.check(); err != nil {
		t.Fatal(err)
	}
	host := map[string]hostEntry{}
	if err := strict(filepath.Join(docs, "agents.json.dist"), &host); err != nil {
		t.Fatal(err)
	}
	for key, h := range host {
		if _, why := limitsFor(installed{Key: key, Host: h, Agent: a}); why != "" {
			t.Errorf("%s: %s", key, why)
		}
	}
}
