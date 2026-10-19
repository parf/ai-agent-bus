package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestRunnerAnswersPackageAsksWithoutScript(t *testing.T) {
	r, b, _ := configurationFixture(t, "", "printf script-was-run")
	s := r.served["#fixture@h"]
	s.in.Agent.Description = "Manifest fixture"
	s.in.Agent.Methods = map[string]agentmeta.Method{"read": {Help: "Read facts"}}
	if err := os.WriteFile(filepath.Join(s.dir, "env.dist"), []byte("# Token purpose\nTOKEN=sentinel-no-output\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, ask := range []string{protocol.AskHelp, protocol.AskHelpConfig, protocol.AskHelpSecret, protocol.AskHelpJSON} {
		r.runOne(s, protocol.Envelope{ID: "fixture", From: "owner@h", To: s.name, Ask: ask})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		e, err := b.Consume(ctx, "owner@h", "", "", false, false)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if e.Receipt != "" || strings.Contains(e.Body, "script-was-run") || strings.Contains(e.Body, "sentinel-no-output") {
			t.Fatalf("package ask ran script or leaked: %+v", e)
		}
		if json.Valid([]byte(e.Body)) != (ask == protocol.AskHelpJSON) {
			t.Fatalf("wrong answer form for %s: %s", ask, e.Body)
		}
		want := map[string]string{"help": "Manifest fixture", "help-config": "No Configuration keys", "help-secret": "Token purpose", "help-json": `"methods"`}[ask]
		if !strings.Contains(e.Body, want) {
			t.Fatalf("%s: %s", ask, e.Body)
		}
	}
}
func TestSelfServingDistDescribesWithoutInjectingOrRequiring(t *testing.T) {
	r, _, _ := configurationFixture(t, "", "true")
	s := r.served["#fixture@h"]
	s.in.Agent.Serves = "itself"
	if err := os.WriteFile(filepath.Join(s.dir, "env.dist"), []byte("# Required but self-serving\nREQUIRED=\nDEFAULT=metadata-only\n"), 0600); err != nil {
		t.Fatal(err)
	}
	env, why := r.environment(s.in, s.bus)
	if why != "" {
		t.Fatalf("metadata requirements enforced: %s", why)
	}
	for _, kv := range env {
		if strings.HasPrefix(kv, "REQUIRED=") || strings.HasPrefix(kv, "DEFAULT=") {
			t.Fatalf("metadata default injected: %s", kv)
		}
	}
}
