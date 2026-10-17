package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Selecting a release that lacks one of today's commands — a rollback past
// the runner's arrival — leaves no link to nothing.
func TestALinkToACommandTheReleaseLacksIsNotLeft(t *testing.T) {
	root, bin := withInstallPaths(t)
	old := filepath.Join(root, "releases", "0.8.100-old")
	for name, target := range commandLinks() {
		if name == "agent-bus-runner" {
			continue
		}
		rel, _ := filepath.Rel(filepath.Join(root, "current"), target)
		path := filepath.Join(old, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := installCommandLinks(); err != nil {
		t.Fatal(err)
	}
	if _, err := selectBundle("0.8.100-old"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(bin, "agent-bus-runner")); !os.IsNotExist(err) {
		t.Fatal("a link to the command the release lacks was left")
	}
	if _, err := os.Stat(filepath.Join(bin, "agent-bus")); err != nil {
		t.Fatalf("the release's own command does not resolve: %v", err)
	}
}
