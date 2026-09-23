package api

import (
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// known makes each name a principal: a name spelled as an Agent's is an Agent
// owned by fixtureOwner, and any other name is a User with the user record
// every User has. Stated the way a store loaded at start states it, so a
// fixture does not need the ceremony of creating each.
func known(t *testing.T, b *core.Bus, names ...string) {
	t.Helper()
	s := ports.Snapshot{Clean: true}
	user := func(name string) {
		s.Users = append(s.Users, protocol.User{Name: name, Status: "active"})
		s.Records = append(s.Records, protocol.Record{
			Name: name, Owner: name, Kind: protocol.KindUser, Personal: true,
			Full: protocol.OverflowStrict, At: time.Now(),
		})
	}
	agents := false
	for _, name := range names {
		n, err := protocol.ParseName(name)
		if err != nil {
			t.Fatalf("fixture principal %s: %v", name, err)
		}
		if n.Agent {
			agents = true
			s.Records = append(s.Records, protocol.Record{
				Name: n.String(), Owner: fixtureOwner, Kind: protocol.KindAgent,
				Full: protocol.OverflowStrict, At: time.Now(),
			})
			continue
		}
		user(n.String())
	}
	if agents && !b.IsPerson(fixtureOwner) {
		user(fixtureOwner)
	}
	b.Restore(s)
}

// fixtureOwner is the User a fixture's agents belong to.
const fixtureOwner = "fixture-owner@h"
