package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestSetupDoesNotManageExternalQdrant(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "setup")
	if out, err := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	help, err := exec.CommandContext(ctx, binary, "--help").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(help)), "qdrant") {
		t.Fatal("setup advertises management of an external store")
	}
	out, err := exec.CommandContext(ctx, binary, "--qdrant", "install", "--dry-run").CombinedOutput()
	if err == nil || !strings.Contains(string(out), "flag provided but not defined: -qdrant") {
		t.Fatalf("removed Qdrant flag was accepted: %v %s", err, out)
	}
}

// J31: --help named no option, because it answered before any was defined.
func TestSetupHelpListsItsOptions(t *testing.T) {
	// The build has no deadline of its own: a cold build cache is slow, and
	// only the help answer is bounded.
	binary := filepath.Join(t.TempDir(), "setup")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--help")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil || stderr.Len() != 0 {
		t.Fatalf("--help: %v, stderr %q", err, stderr.String())
	}
	for _, option := range []string{"--owner", "--addr", "--dry-run", "--upgrade", "--runner", "--user-runner", "--user", "--tls", "--tls-name"} {
		if !regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(option) + `( |$)`).Match(out) {
			t.Errorf("--help does not list %s:\n%s", option, out)
		}
	}
}
