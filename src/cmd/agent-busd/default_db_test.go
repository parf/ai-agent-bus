package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/store/sqlite"
)

// The database's default home follows XDG_STATE_HOME as the CLI's own state
// directory does (J48); without it, or with a relative one, which the XDG
// specification says to ignore, it is under the user's home.
func TestDefaultDatabaseFollowsXDGStateHome(t *testing.T) {
	state, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	underHome := filepath.Join(home, ".local", "state", "agent-bus", "agent-bus.db")
	for _, tc := range []struct{ name, xdg, want string }{
		{"absolute", state, filepath.Join(state, "agent-bus", "agent-bus.db")},
		{"unset", "", underHome},
		{"relative", "relative/state", underHome},
	} {
		t.Setenv("XDG_STATE_HOME", tc.xdg)
		if got := defaultDB(); got != tc.want {
			t.Errorf("%s XDG_STATE_HOME: the default database is %s, want %s", tc.name, got, tc.want)
		}
	}
}

// A missing database at the new default still refuses the start; and when one
// sits at the old home default the refusal names it, so the operator can pass
// --db instead of finding an empty node.
func TestMissingDefaultDatabaseRefusesAndNamesTheOldOne(t *testing.T) {
	state, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", state)
	if _, err := openDatabase(defaultDB(), false); !errors.Is(err, sqlite.ErrMissing) || strings.Contains(err.Error(), "previous default") {
		t.Fatalf("no database anywhere: %v, want the plain missing refusal", err)
	}
	if err := os.MkdirAll(filepath.Dir(homeDB()), 0o700); err != nil {
		t.Fatal(err)
	}
	st, err := sqlite.Open(homeDB(), true)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	_, err = openDatabase(defaultDB(), false)
	if !errors.Is(err, sqlite.ErrMissing) || !strings.Contains(err.Error(), homeDB()) {
		t.Fatalf("a database at the old default: %v, want a refusal naming %s", err, homeDB())
	}
	if _, err := os.Stat(defaultDB()); err == nil {
		t.Fatal("the refused start created a database at the new default")
	}
}
