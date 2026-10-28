package main

import (
	"context"
	"os/exec"
	"path/filepath"
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
