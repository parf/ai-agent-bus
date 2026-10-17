package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Two starts of one name cannot both get past the lock, so the loser never
// removes the winner's note; the lock is free again once the holder lets go.
func TestOneNameIsStartedOnceAtATime(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	first, err := hold("#race@h")
	if err != nil {
		t.Fatal(err)
	}
	if second, err := hold("#race@h"); err == nil {
		second.Close()
		t.Fatal("a second start of the same name got the lock")
	} else if !strings.Contains(err.Error(), "already being started or run here") {
		t.Fatalf("the refusal does not say why: %v", err)
	}
	if other, err := hold("#other@h"); err != nil {
		t.Fatalf("another name was held up: %v", err)
	} else {
		other.Close()
	}
	first.Close()
	again, err := hold("#race@h")
	if err != nil {
		t.Fatalf("the lock was not given back: %v", err)
	}
	again.Close()
}

// An agent started by a binary from before the lock holds a note and no lock:
// a start that gets the lock still finds the note and refuses.
func TestALockFreeAgentWithANoteIsStillRunning(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	// No daemon: a start that got past the note would fail reaching it.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fixture: no daemon here", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	saved := transport
	defer func() { transport = saved }()
	transport = func() (*http.Client, string) { return server.Client(), server.URL }
	t.Setenv("AGENT_BUS_TOKEN", "fixture")
	if err := (running{Name: "#old@h", PID: os.Getpid()}).note(); err != nil {
		t.Fatal(err)
	}
	err := start([]string{"#old@h", "/bin/echo"})
	if err == nil || !strings.Contains(err.Error(), "already running here as pid") {
		t.Fatalf("a running agent without the lock was started again: %v", err)
	}
}
