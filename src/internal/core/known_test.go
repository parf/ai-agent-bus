package core

import (
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// known makes each name a principal with an explicitly shared inbox.
//
// Stated rather than performed: no ordinary call brings a self-owned name into
// existence any more. Registering one requires an owner who may already act,
// and a name that is nobody cannot be its own — which leaves enrolment, which
// has proved a key, and user creation, which is somebody deciding. A fixture
// that wants the state without the ceremony writes the state, the same way a
// store loaded at start does.
func known(t *testing.T, b *Bus, names ...string) {
	t.Helper()
	records := make([]protocol.Record, 0, len(names))
	for _, name := range names {
		records = append(records, protocol.Record{Name: name, Kind: "agent", Allow: []string{"*"}})
	}
	provision(t, b, records...)
}

// provision is the same for a record that carries settings a bare principal
// does not — an overflow policy, a mode — which a fixture would otherwise have
// to register to get.
func provision(t *testing.T, b *Bus, records ...protocol.Record) {
	t.Helper()
	s := ports.Snapshot{Clean: true}
	for _, r := range records {
		n, err := canon(r.Name)
		if err != nil {
			t.Fatalf("fixture principal %s: %v", r.Name, err)
		}
		r.Name, r.At = n, time.Now()
		if r.Owner == "" {
			r.Owner = n
		}
		if r.Kind == "" {
			r.Kind = protocol.KindAgent
		}
		if r.Full == "" {
			r.Full = protocol.OverflowStrict
		}
		s.Records = append(s.Records, r)
	}
	b.Restore(s)
	// A record has an inbox from the moment it exists, which registering does
	// and restoring a record with no queue behind it does not.
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range s.Records {
		b.ensure(r.Name)
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
