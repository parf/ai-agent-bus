package core

import (
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// known makes each name a principal with an explicitly shared inbox: a name
// spelled as an Agent's is an Agent owned by the fixture's user, and any other
// name is a User with its own user record.
//
// Stated rather than performed: the state is written the way a store loaded at
// start supplies it, so a fixture does not need the ceremony of creating each.
func known(t *testing.T, b *Bus, names ...string) {
	t.Helper()
	var users []string
	records := make([]protocol.Record, 0, len(names))
	for _, name := range names {
		if protocol.IsAgentName(name) {
			records = append(records, protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: fixtureOwner, Allow: []string{"*"}})
		} else {
			users = append(users, name)
			records = append(records, protocol.Record{Name: name, Kind: protocol.KindUser, Owner: name, Personal: true, Allow: []string{"*"}})
		}
	}
	provision(t, b, append(users, fixtureOwner), records...)
}

// fixtureOwner is the User a fixture's agents belong to.
const fixtureOwner = "fixture-owner@h"

// provision writes records, and the users they need, the way a store loaded at
// start supplies them. Every user gets its own user record; a record with no
// owner belongs to fixtureOwner.
func provision(t *testing.T, b *Bus, users []string, records ...protocol.Record) {
	t.Helper()
	if err := provisionState(b, users, records...); err != nil {
		t.Fatal(err)
	}
}

func provisionState(b *Bus, users []string, records ...protocol.Record) error {
	for _, r := range records {
		if r.Owner == "" || r.Owner == fixtureOwner {
			users = append(users, fixtureOwner)
			break
		}
	}
	s := ports.Snapshot{Clean: true}
	b.mu.Lock()
	for name, u := range b.users {
		s.Users = append(s.Users, u)
		_ = name
	}
	for _, r := range b.records {
		s.Records = append(s.Records, r)
	}
	b.mu.Unlock()
	have := map[string]bool{}
	for _, r := range s.Records {
		have[r.Name] = true
	}
	for _, u := range users {
		n, err := canon(u)
		if err != nil {
			return err
		}
		s.Users = append(s.Users, protocol.User{Name: n, Status: "active"})
		if !have[n] {
			s.Records = append(s.Records, protocol.Record{Name: n, Kind: protocol.KindUser, Owner: n, Personal: true, Full: protocol.OverflowStrict, At: time.Now()})
			have[n] = true
		}
	}
	for _, r := range records {
		n, err := canon(r.Name)
		if err != nil {
			return err
		}
		r.Name, r.At = n, time.Now()
		if r.Owner == "" {
			r.Owner = fixtureOwner
		}
		if r.Kind == "" {
			r.Kind = protocol.KindAgent
		}
		if r.Full == "" && r.Kind != protocol.KindService {
			r.Full = protocol.OverflowStrict
		}
		replaced := false
		for i := range s.Records {
			if s.Records[i].Name == n {
				s.Records[i], replaced = r, true
			}
		}
		if !replaced {
			s.Records = append(s.Records, r)
		}
	}
	b.Restore(s)
	// A record has an inbox from the moment it exists, which registering does
	// and restoring a record with no queue behind it does not.
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range b.records {
		if onBus(r) {
			b.ensure(r.Name)
		}
	}
	return nil
}

// person is the other half of being known: a registered user, with the user
// record every User has and nothing else.
func person(b *Bus, names ...string) {
	if err := provisionState(b, names); err != nil {
		panic(err)
	}
}

// withUsers adds each name to a snapshot as a User with its own user record:
// the least a stored state needs for the records those Users own to load.
func withUsers(s ports.Snapshot, names ...string) ports.Snapshot {
	for _, n := range names {
		s.Users = append(s.Users, protocol.User{Name: n, Status: "active"})
		s.Records = append(s.Records, userRecord(n))
	}
	return s
}

// userRecord is a User's own record, as every User has one.
func userRecord(name string) protocol.Record {
	return protocol.Record{Name: name, Owner: name, Kind: protocol.KindUser, Personal: true, Full: protocol.OverflowStrict}
}
