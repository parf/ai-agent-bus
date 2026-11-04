package core

import (
	"reflect"
	"sort"
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
			records = append(records, protocol.Record{Name: name, Kind: protocol.KindUser, Owner: name, Tags: []string{"personal"}})
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
	have, held := map[string]bool{}, map[string]bool{}
	for _, r := range s.Records {
		have[r.Name] = true
	}
	for _, u := range s.Users {
		held[u.Name] = true
	}
	for _, u := range users {
		n, err := canon(u)
		if err != nil {
			return err
		}
		// A User already here stays who it is: a second, unnumbered copy
		// would be a User recreated under the name, which owns nothing of
		// the first's.
		if held[n] {
			continue
		}
		held[n] = true
		s.Users = append(s.Users, protocol.User{Name: n, Status: "active"})
		if !have[n] {
			s.Records = append(s.Records, protocol.Record{Name: n, Kind: protocol.KindUser, Owner: n, Tags: []string{"personal"}, Full: protocol.OverflowStrict, At: time.Now()})
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
		if r.Full == "" && onBus(r) && r.Kind != protocol.KindPubSub {
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
	s = canonicalFixture(s, b.admin)
	if err := b.Restore(s); err != nil {
		return err
	}
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
	return canonicalFixture(s, s.Owner)
}

// userRecord is a User's own record, as every User has one.
func userRecord(name string) protocol.Record {
	return protocol.Record{Name: name, Owner: name, Kind: protocol.KindUser, Tags: []string{"personal"}, Full: protocol.OverflowStrict}
}

func canonicalFixture(s ports.Snapshot, owner string) ports.Snapshot {
	if owner == "" && len(s.Users) > 0 {
		owner = s.Users[0].Name
	}
	s.Owner, s.OwnerEstablished, s.AccountsEstablished = owner, true, true
	sort.Slice(s.Users, func(i, j int) bool { return s.Users[i].Name < s.Users[j].Name })
	nextUser, nextRecord := uint32(1), uint32(1)
	for _, u := range s.Users {
		if u.ID >= nextUser {
			nextUser = u.ID + 1
		}
	}
	for _, r := range s.Records {
		if r.ID >= nextRecord {
			nextRecord = r.ID + 1
		}
	}
	ids := map[string]uint32{}
	for i := range s.Users {
		if s.Users[i].ID == 0 {
			s.Users[i].ID = nextUser
			nextUser++
		}
		ids[s.Users[i].Name] = s.Users[i].ID
	}
	for i := range s.Records {
		if s.Records[i].ID == 0 {
			s.Records[i].ID = nextRecord
			nextRecord++
		}
		if s.Records[i].OwnerID == 0 {
			s.Records[i].OwnerID = ids[s.Records[i].Owner]
		}
	}
	if nextUser > s.NextUserID {
		s.NextUserID = nextUser
	}
	if nextRecord > s.NextRecordID {
		s.NextRecordID = nextRecord
	}
	return s
}

func assertRestoreRefused(t *testing.T, b *Bus, s ports.Snapshot) {
	t.Helper()
	before := b.Snapshot()
	if err := b.Restore(s); err == nil {
		t.Fatal("unsupported snapshot accepted")
	}
	after := b.Snapshot()
	before.At, after.At = time.Time{}, time.Time{}
	for _, snap := range []*ports.Snapshot{&before, &after} {
		sort.Slice(snap.Users, func(i, j int) bool { return snap.Users[i].Name < snap.Users[j].Name })
		sort.Slice(snap.Records, func(i, j int) bool { return snap.Records[i].Name < snap.Records[j].Name })
		sort.Slice(snap.Queues, func(i, j int) bool { return snap.Queues[i].Name < snap.Queues[j].Name })
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("refused snapshot changed existing state")
	}
}
