package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Legacy services/ notes are no longer a supported process identity.
func TestLegacyNotesAreNotProcessIdentity(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	write := func(path string, pid int) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(running{Name: strings.TrimSuffix(filepath.Base(path), ".json"), PID: pid})
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(notePath("#new@h"), string(filepath.Separator)+"agents"+string(filepath.Separator)) {
		t.Fatalf("a new note is not under agents/: %s", notePath("#new@h"))
	}
	legacy := filepath.Join(runDir(), "services", segment("#old@h")+".json")
	write(legacy, os.Getpid())
	if _, err := alive("#old@h"); err == nil {
		t.Fatal("legacy note accepted as current process identity")
	}
	write(notePath("#new@h"), os.Getpid())
	if _, err := alive("#new@h"); err != nil {
		t.Fatal(err)
	}
}
