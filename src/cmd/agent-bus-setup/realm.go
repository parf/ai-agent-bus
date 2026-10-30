package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// realmBus is the part of the Owner's connection the automatic runner names
// need.
type realmBus interface {
	call(method, path string, body any) (int, []byte, error)
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

// automaticRealm is the realm of a person's runner name. A runner already
// registered, under whatever realm, is kept and reused: a second record would
// be a second identity and orphan the first one's token. Two or more are
// refused by name; none takes the bus's realm. suffix is "" for the system
// runner and "-user" for the user-account one.
func automaticRealm(bus realmBus, person, suffix string) (string, error) {
	prefix := "#runner/" + person + suffix + "@"
	code, out, err := bus.call("GET", "/ls?kind=agent&q="+url.QueryEscape("#runner/"+person), nil)
	if err != nil || code != http.StatusOK {
		return "", fmt.Errorf("list the runners of %s: %d %v", person, code, err)
	}
	var rows []struct{ Name, Owner string }
	if err := json.Unmarshal(out, &rows); err != nil {
		return "", fmt.Errorf("list the runners of %s: %w", person, err)
	}
	var found []string
	for _, r := range rows {
		if strings.HasPrefix(r.Name, prefix) && r.Owner == person {
			found = append(found, r.Name)
		}
	}
	switch len(found) {
	case 0:
		return busRealm(bus)
	case 1:
		return strings.TrimPrefix(found[0], prefix), nil
	}
	return "", fmt.Errorf("%s has more than one runner record (%s); remove all but one before installing", person, strings.Join(found, ", "))
}
