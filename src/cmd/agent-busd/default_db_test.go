package main

import (
	"path/filepath"
	"testing"
)

// The database's default home follows XDG_STATE_HOME as the CLI's own state
// directory does (J48); without it, it is under the user's home.
func TestDefaultDatabaseFollowsXDGStateHome(t *testing.T) {
	state, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", state)
	if got, want := defaultDB(), filepath.Join(state, "agent-bus", "agent-bus.db"); got != want {
		t.Fatalf("with XDG_STATE_HOME set the default database is %s, want %s", got, want)
	}
	t.Setenv("XDG_STATE_HOME", "")
	if got, want := defaultDB(), filepath.Join(home, ".local", "state", "agent-bus", "agent-bus.db"); got != want {
		t.Fatalf("without it the default database is %s, want %s", got, want)
	}
}
