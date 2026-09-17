package main

import (
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// known makes each name a principal that answers for itself, without a profile.
//
// Stated rather than registered. No ordinary call brings a self-owned name
// into existence any more: registering one needs an owner who may already act,
// and a name that is nobody cannot be its own. What still supplies this shape
// is a store loaded at start, and enrolment, so a fixture says it the way a
// store does. See docs/01-identity-and-roles.md#registration.
func known(t *testing.T, b *core.Bus, names ...string) {
	t.Helper()
	s := ports.Snapshot{Clean: true}
	for _, name := range names {
		n, err := protocol.ParseName(name)
		if err != nil {
			t.Fatalf("fixture principal %s: %v", name, err)
		}
		s.Records = append(s.Records, protocol.Record{
			Name: n.String(), Owner: n.String(), Kind: "agent",
			Full: protocol.OverflowStrict, At: time.Now(),
		})
	}
	b.Restore(s)
}
