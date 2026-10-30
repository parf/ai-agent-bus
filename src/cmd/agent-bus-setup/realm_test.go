package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// fakeRealmBus answers /status with the realm it holds and a lookup from the
// names it has registered.
type fakeRealmBus struct {
	realm      string
	registered map[string]bool
}

func (f fakeRealmBus) call(method, path string, body any) (int, []byte, error) {
	out, _ := json.Marshal(map[string]string{"realm": f.realm})
	return http.StatusOK, out, nil
}

func (f fakeRealmBus) lookup(name string) (map[string]any, bool, error) {
	if f.registered[name] {
		return map[string]any{"name": name}, true, nil
	}
	return nil, false, nil
}

func TestPersonRunnerNameTakesTheBusRealmNotTheHost(t *testing.T) {
	bus := fakeRealmBus{realm: "parf.us"}
	p, err := namedRunner(bus, "alice", "node7.example")
	if err != nil || p.name != "#runner/alice@parf.us" {
		t.Fatalf("new runner = %q, %v; want the bus realm", p.name, err)
	}
	// An existing runner keeps the name it was registered under.
	bus.registered = map[string]bool{"#runner/alice@node7.example": true}
	if p, err = namedRunner(bus, "alice", "node7.example"); err != nil || p.name != "#runner/alice@node7.example" {
		t.Fatalf("registered runner = %q, %v; its name must not change", p.name, err)
	}
	// A bus that names no realm is not answered with the host.
	if _, err = namedRunner(fakeRealmBus{}, "alice", "node7.example"); err == nil || !strings.Contains(err.Error(), "reports no realm") {
		t.Fatalf("realm-less bus: %v", err)
	}
}

func TestUserRunnerNameTakesTheBusRealmNotTheHost(t *testing.T) {
	fakeAccounts(t, map[string]account{"alice": {uid: 1001, gid: 1001, home: "/home/alice", shell: "/bin/bash"}})
	bus := fakeRealmBus{realm: "parf.us"}
	u, err := namedUserRunner(bus, "alice", "node7.example")
	if err != nil || u.name != "#runner/alice-user@parf.us" {
		t.Fatalf("new runner = %q, %v; want the bus realm", u.name, err)
	}
	bus.registered = map[string]bool{"#runner/alice-user@node7.example": true}
	if u, err = namedUserRunner(bus, "alice", "node7.example"); err != nil || u.name != "#runner/alice-user@node7.example" {
		t.Fatalf("registered runner = %q, %v; its name must not change", u.name, err)
	}
	if _, err = namedUserRunner(fakeRealmBus{}, "alice", "node7.example"); err == nil || !strings.Contains(err.Error(), "reports no realm") {
		t.Fatalf("realm-less bus: %v", err)
	}
}
