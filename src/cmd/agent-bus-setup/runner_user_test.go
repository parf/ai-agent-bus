package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
)

// Setup gives the runner its User through the admin program and restarts the
// daemon only when the account map changed, which is what serves the socket.
func TestSetupGivesTheRunnerItsUser(t *testing.T) {
	dir := t.TempDir()
	asked := filepath.Join(dir, "asked")
	admin := func(answer string, code int) string {
		path := filepath.Join(dir, "admin")
		script := "#!/bin/sh\necho \"$@\" >" + asked + "\necho '" + answer + "'\nexit " + string(rune('0'+code)) + "\n"
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	for _, tc := range []struct {
		answer  string
		restart bool
	}{
		{api.RestartForSockets, true},
		{"account map unchanged", false},
	} {
		restarted := false
		if err := runnerUser(admin(tc.answer, 0), func() error { restarted = true; return nil }); err != nil {
			t.Fatal(err)
		}
		if restarted != tc.restart {
			t.Fatalf("%q: restarted=%v, want %v", tc.answer, restarted, tc.restart)
		}
		got, _ := os.ReadFile(asked)
		if strings.TrimSpace(string(got)) != "user add-local agent-bus-runner agent-bus-runner" {
			t.Fatalf("admin was asked %q", got)
		}
	}
	if err := runnerUser(admin("refused", 1), func() error { return nil }); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("a refusal was not said: %v", err)
	}
}
