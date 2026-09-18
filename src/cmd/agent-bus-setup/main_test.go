package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUnitDelegatesOnlyTheWebControllers(t *testing.T) {
	unit := unitFor("/program/agent-busd", "127.0.0.1:6767", "owner@example", nil)
	for _, want := range []string{
		"Delegate=cpu memory pids",
		"DelegateSubgroup=supervisor",
		"CapabilityBoundingSet=CAP_CHOWN",
		" -web",
	} {
		if strings.Count(unit, want) != 1 {
			t.Errorf("generated unit has %d copies of %q", strings.Count(unit, want), want)
		}
	}
}

func TestFirstUserWaitsForTheCredentialSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "account.sock")
	ready := make(chan net.Listener, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		l, err := net.Listen("unix", path)
		if err != nil {
			ready <- nil
			return
		}
		ready <- l
	}()
	if err := waitForSocket(path, time.Second); err != nil {
		t.Fatal(err)
	}
	l := <-ready
	if l == nil {
		t.Fatal("fixture listener failed")
	}
	l.Close()

	regular := filepath.Join(t.TempDir(), "not-a-socket")
	if err := os.WriteFile(regular, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := waitForSocket(regular, 25*time.Millisecond); err == nil {
		t.Fatal("regular file satisfied the credential-socket wait")
	}
}
