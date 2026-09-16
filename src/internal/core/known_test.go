package core

import (
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// known makes each name a principal that answers for itself, which is what the
// daemon now requires before a name may own anything
// (docs/01-identity.md#registration). Fixtures that wanted an owner used to be
// able to name a string nobody had heard of; a record owned by nobody the
// daemon knows is the wreckage the deletion rule exists to clean up, so it can
// no longer be registered into existence.
func known(t *testing.T, b *Bus, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := b.Register(protocol.Record{Name: name, Owner: name, Kind: "agent"}); err != nil {
			t.Fatalf("fixture principal %s: %v", name, err)
		}
	}
}

// person is the other half of being known: a registered user, with no record
// of its own. Fixtures that count records want this one, because registering a
// principal to make it known would add a record to the count under test.
func person(b *Bus, names ...string) {
	s := ports.Snapshot{Clean: true}
	for _, name := range names {
		s.Users = append(s.Users, protocol.User{Name: name, State: "active"})
	}
	b.Restore(s)
}
