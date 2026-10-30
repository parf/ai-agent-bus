package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// fakeRealmBus answers /status with the realm it holds and /ls with the
// agent records it has registered, each owned by its person.
type fakeRealmBus struct {
	realm      string
	registered []string
	foreign    []string // records under a person's runner name that another principal owns
}

func (f fakeRealmBus) call(method, path string, body any) (int, []byte, error) {
	if strings.HasPrefix(path, "/ls") {
		rows := []map[string]string{{"name": "#runner/bobby@other", "owner": "bobby"}}
		for _, n := range f.registered {
			owner := strings.TrimSuffix(strings.SplitN(strings.TrimPrefix(n, "#runner/"), "@", 2)[0], "-user")
			rows = append(rows, map[string]string{"name": n, "owner": owner})
		}
		for _, n := range f.foreign {
			rows = append(rows, map[string]string{"name": n, "owner": "mallory"})
		}
		out, _ := json.Marshal(rows)
		return http.StatusOK, out, nil
	}
	out, _ := json.Marshal(map[string]string{"realm": f.realm})
	return http.StatusOK, out, nil
}

func TestPersonRunnerNameFollowsTheRecordOrTheBusRealm(t *testing.T) {
	// none: created at the bus realm, not the host's
	p, err := namedRunner(fakeRealmBus{realm: "parf.us"}, "alice")
	if err != nil || p.name != "#runner/alice@parf.us" {
		t.Fatalf("new runner = %q, %v; want the bus realm", p.name, err)
	}
	// exactly one, under any realm (host name or an older bus realm): kept
	for _, old := range []string{"node7.example", "older.test"} {
		bus := fakeRealmBus{realm: "parf.us", registered: []string{"#runner/alice@" + old, "#runner/alice-user@user.realm"}}
		if p, err = namedRunner(bus, "alice"); err != nil || p.name != "#runner/alice@"+old {
			t.Fatalf("registered runner = %q, %v; its name must not change", p.name, err)
		}
	}
	// a record another principal owns is not hers: it neither counts nor is reused
	if p, err = namedRunner(fakeRealmBus{realm: "parf.us", foreign: []string{"#runner/alice@squat.test"}}, "alice"); err != nil || p.name != "#runner/alice@parf.us" {
		t.Fatalf("foreign-owned record = %q, %v", p.name, err)
	}
	// more than one: refused by name, nothing created
	two := fakeRealmBus{realm: "parf.us", registered: []string{"#runner/alice@one.test", "#runner/alice@two.test"}}
	if _, err = namedRunner(two, "alice"); err == nil || !strings.Contains(err.Error(), "#runner/alice@one.test") || !strings.Contains(err.Error(), "#runner/alice@two.test") {
		t.Fatalf("two records: %v", err)
	}
	// a bus that names no realm, and no record: refused, not answered with the host
	if _, err = namedRunner(fakeRealmBus{}, "alice"); err == nil || !strings.Contains(err.Error(), "reports no realm") {
		t.Fatalf("realm-less bus: %v", err)
	}
	// an existing record needs no realm from the bus
	if p, err = namedRunner(fakeRealmBus{registered: []string{"#runner/alice@one.test"}}, "alice"); err != nil || p.name != "#runner/alice@one.test" {
		t.Fatalf("registered runner on a realm-less bus = %q, %v", p.name, err)
	}
}

func TestUserRunnerNameFollowsTheRecordOrTheBusRealm(t *testing.T) {
	fakeAccounts(t, map[string]account{"alice": {uid: 1001, gid: 1001, home: "/home/alice", shell: "/bin/bash"}})
	u, err := namedUserRunner(fakeRealmBus{realm: "parf.us"}, "alice")
	if err != nil || u.name != "#runner/alice-user@parf.us" {
		t.Fatalf("new runner = %q, %v; want the bus realm", u.name, err)
	}
	for _, old := range []string{"node7.example", "older.test"} {
		bus := fakeRealmBus{realm: "parf.us", registered: []string{"#runner/alice-user@" + old, "#runner/alice@system.realm"}}
		if u, err = namedUserRunner(bus, "alice"); err != nil || u.name != "#runner/alice-user@"+old {
			t.Fatalf("registered runner = %q, %v; its name must not change", u.name, err)
		}
	}
	two := fakeRealmBus{realm: "parf.us", registered: []string{"#runner/alice-user@one.test", "#runner/alice-user@two.test"}}
	if _, err = namedUserRunner(two, "alice"); err == nil || !strings.Contains(err.Error(), "one.test") || !strings.Contains(err.Error(), "two.test") {
		t.Fatalf("two records: %v", err)
	}
	if _, err = namedUserRunner(fakeRealmBus{}, "alice"); err == nil || !strings.Contains(err.Error(), "reports no realm") {
		t.Fatalf("realm-less bus: %v", err)
	}
}
