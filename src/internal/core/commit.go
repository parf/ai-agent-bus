package core

import (
	"fmt"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A management write is validated, committed to the store as one
// transaction, and only then answered. It is staged in the live maps under
// b.mu, which every reader also takes, so nobody sees it before the commit;
// if the commit fails, the staged entities are put back from the undo log and
// nothing was ever visible (docs/constitution.md#persistence-and-loading).
//
// Every write to a durable map goes through the helpers below, which is what
// lets a commit name exactly the entities it changed: an edit of one record
// writes one record, never the registry.

// staged is the undo log of the write in progress: each entity's value before
// its first change, or absent when it did not exist.
type staged struct {
	records    map[string]*protocol.Record
	users      map[string]*protocol.User
	groups     map[string]*[]string
	inboxes    map[string]*inbox
	owner      *string
	accounts   map[string]string
	accountsOn bool
}

func (s *staged) empty() bool {
	return len(s.records) == 0 && len(s.users) == 0 && len(s.groups) == 0 &&
		len(s.inboxes) == 0 && s.owner == nil && !s.accountsOn
}

func (b *Bus) stage() *staged {
	if b.staging == nil {
		b.staging = &staged{
			records: map[string]*protocol.Record{},
			users:   map[string]*protocol.User{},
			groups:  map[string]*[]string{},
			inboxes: map[string]*inbox{},
		}
	}
	return b.staging
}

// setRecord and the helpers beside it are the only writers of durable maps
// outside Restore. Caller holds b.mu.
func (b *Bus) setRecord(name string, r protocol.Record) {
	s := b.stage()
	if _, seen := s.records[name]; !seen {
		if old, had := b.records[name]; had {
			s.records[name] = &old
		} else {
			s.records[name] = nil
		}
	}
	b.records[name] = r
}

func (b *Bus) dropRecord(name string) {
	if _, had := b.records[name]; !had {
		return
	}
	s := b.stage()
	if _, seen := s.records[name]; !seen {
		old := b.records[name]
		s.records[name] = &old
	}
	delete(b.records, name)
}

// dropInbox removes a name's queue with its durable state.
func (b *Bus) dropInbox(name string) {
	in, had := b.inboxes[name]
	if !had {
		return
	}
	s := b.stage()
	if _, seen := s.inboxes[name]; !seen {
		s.inboxes[name] = in
	}
	delete(b.inboxes, name)
}

func (b *Bus) setUser(name string, u protocol.User) {
	s := b.stage()
	if _, seen := s.users[name]; !seen {
		if old, had := b.users[name]; had {
			s.users[name] = &old
		} else {
			s.users[name] = nil
		}
	}
	b.users[name] = u
}

func (b *Bus) setGroup(name string, members []string) {
	s := b.stage()
	if _, seen := s.groups[name]; !seen {
		if old, had := b.groups[name]; had {
			kept := append([]string(nil), old...)
			s.groups[name] = &kept
		} else {
			s.groups[name] = nil
		}
	}
	b.groups[name] = members
}

func (b *Bus) setOwner(owner string) {
	s := b.stage()
	if s.owner == nil {
		old := b.admin
		s.owner = &old
	}
	b.admin = owner
}

func (b *Bus) setAccounts(accounts map[string]string) {
	s := b.stage()
	if !s.accountsOn {
		s.accounts, s.accountsOn = cloneAccounts(b.accounts), true
	}
	b.accounts = accounts
}

// commit writes the staged entities as one transaction. On failure they are
// put back, so the error is the whole outcome: nothing changed. Without a
// store (tests, embedding) the write simply stands. Caller holds b.mu.
func (b *Bus) commit() error {
	s := b.staging
	b.staging = nil
	if s == nil || s.empty() {
		return nil
	}
	if b.store == nil {
		return nil
	}
	if err := b.store.Commit(b.change(s)); err != nil {
		b.rollback(s)
		return fmt.Errorf("persist administrative state: %w", err)
	}
	for name := range s.inboxes {
		delete(b.flushed, name)
	}
	return nil
}

// change is what the staged entities are now.
func (b *Bus) change(s *staged) ports.Change {
	var c ports.Change
	if len(s.records) > 0 {
		c.Records = map[string]*protocol.Record{}
		for name := range s.records {
			if r, has := b.records[name]; has {
				r := r
				clearLiveRecord(&r)
				c.Records[name] = &r
			} else {
				c.Records[name] = nil
			}
		}
	}
	if len(s.users) > 0 {
		c.Users = map[string]*protocol.User{}
		for name := range s.users {
			if u, has := b.users[name]; has {
				u := u
				c.Users[name] = &u
			} else {
				c.Users[name] = nil
			}
		}
	}
	if len(s.groups) > 0 {
		c.Groups = map[string]*[]string{}
		for name := range s.groups {
			if members, has := b.groups[name]; has {
				members := append([]string{}, members...)
				c.Groups[name] = &members
			} else {
				c.Groups[name] = nil
			}
		}
	}
	for name := range s.inboxes {
		if _, back := b.inboxes[name]; !back {
			c.DropQueues = append(c.DropQueues, name)
		}
	}
	if s.owner != nil {
		owner := b.admin
		c.Owner = &owner
	}
	if s.accountsOn {
		c.Accounts = cloneAccounts(b.accounts)
	}
	return c
}

// rollback restores every staged entity to its value before the write.
func (b *Bus) rollback(s *staged) {
	for name, old := range s.records {
		if old == nil {
			delete(b.records, name)
		} else {
			b.records[name] = *old
		}
	}
	for name, old := range s.users {
		if old == nil {
			delete(b.users, name)
		} else {
			b.users[name] = *old
		}
	}
	for name, old := range s.groups {
		if old == nil {
			delete(b.groups, name)
		} else {
			b.groups[name] = *old
		}
	}
	for name, in := range s.inboxes {
		b.inboxes[name] = in
	}
	if s.owner != nil {
		b.admin = *s.owner
	}
	if s.accountsOn {
		b.accounts = s.accounts
	}
}

// discard drops a staged write that failed validation before it reached the
// store, putting everything back. Caller holds b.mu.
func (b *Bus) discard() {
	if s := b.staging; s != nil {
		b.staging = nil
		b.rollback(s)
	}
}
