package api

import (
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// known makes each name a principal: a name spelled as an Agent's is an Agent
// owned by fixtureOwner, and any other name is a User with the user record
// every User has. Stated the way a store loaded at start states it, so a
// fixture does not need the ceremony of creating each.
func known(t testing.TB, b *core.Bus, names ...string) {
	t.Helper()
	owner := b.DaemonOwner()
	if owner == "" {
		owner = fixtureOwner
		b.SetDaemonOwner(owner)
	}
	user := func(name string) {
		if b.IsPerson(name) {
			return
		}
		if _, err := b.SetUser(owner, protocol.User{Name: name, Status: "active"}, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range names {
		n, err := protocol.ParseName(name)
		if err != nil {
			t.Fatal(err)
		}
		if n.Agent {
			user(fixtureOwner)
			if _, err := b.Register(protocol.Record{Name: n.String(), Owner: fixtureOwner, Kind: protocol.KindAgent}); err != nil {
				t.Fatal(err)
			}
		} else {
			user(n.String())
		}
	}
}

// fixtureOwner is the User a fixture's agents belong to.
const fixtureOwner = "fixture-owner@h"
