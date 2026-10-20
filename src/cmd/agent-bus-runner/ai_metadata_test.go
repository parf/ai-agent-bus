package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/agentmeta"
)

func TestAIHostKeyDeclarationReportsMissingKey(t *testing.T) {
	dir := filepath.Join("..", "..", "agents", "ai")
	vars, err := readEnv(filepath.Join(dir, "env.dist"))
	if err != nil {
		t.Fatal(err)
	}
	_, missing := layered(vars)
	if len(missing) != 1 || missing[0] != "OPENROUTER_API_KEY" {
		t.Fatalf("AI must report exactly the missing provider key: %v", missing)
	}
	manifest, err := agentmeta.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, ask := range []string{"help", "help-config", "help-secret", "help-json"} {
		body, err := manifest.Answer(dir, ask, []string{"owner"}, false)
		if err != nil || body == "" {
			t.Fatalf("%s must render package help without a value: %q %v", ask, body, err)
		}
		if ask == "help-secret" && !strings.Contains(body, "OPENROUTER_API_KEY") {
			t.Fatalf("key missing from secret help: %s", body)
		}
	}
}
