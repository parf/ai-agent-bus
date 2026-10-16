package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// service.d became agent.d: setup moves an installed one, keeping what it
// holds, and refuses when both exist rather than drop either.
func TestTheAgentDirectoryKeepsWhatItHeldUnderItsNewName(t *testing.T) {
	root := t.TempDir()
	old, renamed := filepath.Join(root, "service.d"), filepath.Join(root, "agent.d")
	if err := os.MkdirAll(filepath.Join(old, "mail-reader"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !legacyDirPresent(old, renamed) {
		t.Fatal("an installed service.d was not seen as the old name")
	}
	if err := moveLegacyDir(old, renamed); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(renamed, "mail-reader")); err != nil {
		t.Fatalf("agent.d lost what service.d held: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("service.d is still there: %v", err)
	}
	if err := moveLegacyDir(old, renamed); err != nil {
		t.Fatalf("a second run is not a no-op: %v", err)
	}
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := moveLegacyDir(old, renamed); err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("with both present it did not refuse: %v", err)
	}
}
