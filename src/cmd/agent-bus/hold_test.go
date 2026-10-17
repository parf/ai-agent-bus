package main

import (
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
