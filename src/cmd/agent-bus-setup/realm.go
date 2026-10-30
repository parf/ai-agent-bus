package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// realmBus is the part of the Owner's connection the automatic runner names
// need.
type realmBus interface {
	call(method, path string, body any) (int, []byte, error)
	lookup(name string) (map[string]any, bool, error)
}

// busRealm is the realm the bus itself reports (docs/agents/bus.md#configuration),
// read at each use. A bus that names none is not answered with the host name:
// the automatic name is refused.
func busRealm(bus realmBus) (string, error) {
	code, out, err := bus.call("GET", "/status", nil)
	if err != nil || code != http.StatusOK {
		return "", fmt.Errorf("read the bus realm: %d %v", code, err)
	}
	var s struct{ Realm string }
	if err := json.Unmarshal(out, &s); err != nil {
		return "", fmt.Errorf("read the bus realm: %w", err)
	}
	if s.Realm == "" {
		return "", fmt.Errorf("the bus reports no realm, so an automatic runner name is refused; ask the bus Owner to save the #bus setup")
	}
	return s.Realm, nil
}

// automaticRealm is the realm of a runner's name. A runner already registered
// under the host's name keeps it (an existing name never changes); a new one
// takes the bus's realm.
func automaticRealm(bus realmBus, host string, name func(realm string) string) (string, error) {
	if _, ok, err := bus.lookup(name(host)); err != nil {
		return "", err
	} else if ok {
		return host, nil
	}
	return busRealm(bus)
}
