package main

import (
	"net"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/store/sqlite"
)

func TestBusChildRetainsEnvironment(t *testing.T) {
	around := []string{"PATH=/usr/bin", "AGENT_BUS_TOKEN=bus-only"}
	bus := &child{what: "bus", env: []string{roleEnv + "=" + roleBus}}
	got := bus.environ(around)
	for _, want := range append(around, roleEnv+"="+roleBus) {
		if !slices.Contains(got, want) {
			t.Errorf("bus child lost %q", want)
		}
	}
}

func TestSupervisorUsesEstablishedAccountMapInsteadOfSetupFlags(t *testing.T) {
	other, err := user.Lookup("nobody")
	if err != nil {
		t.Skip("no second local account")
	}
	path := filepath.Join(t.TempDir(), "bus.db")
	var seed accounts
	if err := seed.Set(other.Username + "=seed@h"); err != nil {
		t.Fatal(err)
	}
	c := config{db: path, create: true, users: seed}
	got, err := supervisorAccounts(c)
	if err != nil || got.mapping()[other.Username] != "seed@h" {
		t.Fatalf("first-run setup seed: %v, %v", got.mapping(), err)
	}
	storeAccounts(t, path, map[string]string{other.Username: "stored@h"})
	got, err = supervisorAccounts(c)
	if err != nil || got.mapping()[other.Username] != "stored@h" {
		t.Fatalf("stored map did not replace setup flags: %v, %v", got.mapping(), err)
	}
	storeAccounts(t, path, map[string]string{})
	got, err = supervisorAccounts(c)
	if err != nil || len(got.list()) != 0 {
		t.Fatalf("intentionally empty map resurrected setup flags: %v, %v", got.mapping(), err)
	}
}

func TestSupervisorRefusesUnusableStoredAccountMap(t *testing.T) {
	me, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	for _, stored := range []map[string]string{
		{"account-that-does-not-exist-agent-bus": "user@h"},
		{me.Username: "user@h"},
	} {
		path := filepath.Join(t.TempDir(), "bus.db")
		storeAccounts(t, path, stored)
		if _, err := supervisorAccounts(config{db: path}); err == nil {
			t.Fatalf("unusable stored map was accepted: %+v", stored)
		}
	}
	// A missing database is not a first run unless -create said so.
	if _, err := supervisorAccounts(config{db: filepath.Join(t.TempDir(), "absent.db")}); err == nil {
		t.Fatal("a missing database was taken for a first run")
	}
}

func storeAccounts(t *testing.T, path string, accounts map[string]string) {
	t.Helper()
	st, err := sqlite.Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Commit(ports.Change{Accounts: accounts}); err != nil {
		t.Fatal(err)
	}
}

func TestRetiredUserSocketsAreRemovedWithoutTouchingCurrentOnes(t *testing.T) {
	dir := t.TempDir()
	listen := func(name string) net.Listener {
		t.Helper()
		l, err := net.Listen("unix", filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		l.(*net.UnixListener).SetUnlinkOnClose(false)
		return l
	}
	kept, retired := listen("user-kept.sock"), listen("user-retired.sock")
	kept.Close()
	retired.Close()
	if err := clearRetiredUserSockets(dir, map[string]bool{"user-kept.sock": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "user-kept.sock")); err != nil {
		t.Fatal("current mapped socket was removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "user-retired.sock")); !os.IsNotExist(err) {
		t.Fatal("retired socket stayed discoverable")
	}
	if err := os.WriteFile(filepath.Join(dir, "user-collision.sock"), []byte("not a socket"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := clearRetiredUserSockets(dir, map[string]bool{"user-kept.sock": true}); err == nil {
		t.Fatal("a non-socket collision was silently removed")
	}
}

// webChild refuses to exist without a binary beside it, so the test supplies
// one rather than skipping the assertion that matters.
func exeForTest(t *testing.T) (string, error) {
	t.Helper()
	path := t.TempDir() + "/agent-bus-web"
	return path, os.WriteFile(path, []byte("#!/bin/true\n"), 0o755)
}
