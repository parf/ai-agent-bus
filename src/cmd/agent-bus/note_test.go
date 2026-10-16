package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A note moved from services/ to agents/: an agent started by an older binary
// is still found by stop and logs, and a stale one is cleared where it lies.
func TestAnOlderAgentsNoteIsStillFound(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	write := func(path string, pid int) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(running{Name: "#old@h", PID: pid})
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(notePath("#new@h"), string(filepath.Separator)+"agents"+string(filepath.Separator)) {
		t.Fatalf("a new note is not under agents/: %s", notePath("#new@h"))
	}
	write(legacyNotePath("#old@h"), os.Getpid())
	r, err := alive("#old@h")
	if err != nil || r.PID != os.Getpid() {
		t.Fatalf("an older agent's note was not found: %v %+v", err, r)
	}
	// A note whose process is gone is removed from where it was found.
	write(legacyNotePath("#gone@h"), 1<<30)
	if _, err := alive("#gone@h"); err == nil {
		t.Fatal("a dead agent's old note read as running")
	}
	if _, err := os.Stat(legacyNotePath("#gone@h")); !os.IsNotExist(err) {
		t.Fatalf("the stale old note was left behind: %v", err)
	}
}
