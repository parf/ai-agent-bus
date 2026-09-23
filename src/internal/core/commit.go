package core

import (
	"fmt"
	"time"

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
	inboxes    map[string]*inbox
	owner      *string
	accounts   map[string]string
	accountsOn bool
	// The ID high-water marks before the write, when it moved them.
	nextRecordID, nextUserID uint32
	idsOn                    bool
	// creds is the write's credential half: a new pair, or nil to remove.
	// Nothing in memory moves until the commit lands (credentials.go).
	creds map[string]*ports.CredentialPair
}

func (s *staged) empty() bool {
	return len(s.records) == 0 && len(s.users) == 0 &&
		len(s.inboxes) == 0 && s.owner == nil && !s.accountsOn && !s.idsOn && len(s.creds) == 0
}

func (b *Bus) stage() *staged {
	if b.staging == nil {
		b.staging = &staged{
			records: map[string]*protocol.Record{},
			users:   map[string]*protocol.User{},
			inboxes: map[string]*inbox{},
		}
	}
	return b.staging
}

// setRecord and the helpers beside it are the only writers of durable maps
// outside Restore. Caller holds b.mu.
func (b *Bus) setRecord(name string, r protocol.Record) {
	s := b.stage()
	old, had := b.records[name]
	if _, seen := s.records[name]; !seen {
		if had {
			s.records[name] = &old
		} else {
			s.records[name] = nil
		}
	}
	// An existing record keeps its identity and its birth; a new one is
	// given the next ID, which is never handed out again.
	if had {
		r.ID, r.Created = old.ID, old.Created
	} else {
		r.ID = b.takeID(&b.nextRecordID)
		r.Created = time.Now()
	}
	if r.Created.IsZero() {
		r.Created = time.Now()
	}
	b.records[name] = r
	b.recordByID[r.ID] = name
}

// takeID hands out the next ID from a high-water mark and moves the mark,
// staging the old value so a failed write gives it back. The mark never
// wraps: past the last ID there are no more.
func (b *Bus) takeID(next *uint32) uint32 {
	s := b.stage()
	if !s.idsOn {
		s.nextRecordID, s.nextUserID, s.idsOn = b.nextRecordID, b.nextUserID, true
	}
	id := *next
	if id == 0 || id == ^uint32(0) {
		b.idsExhausted = true
		return 0
	}
	*next = id + 1
	return id
}

func (b *Bus) dropRecord(name string) {
	if _, had := b.records[name]; !had {
		return
	}
	s := b.stage()
	old := b.records[name]
	if _, seen := s.records[name]; !seen {
		s.records[name] = &old
	}
	delete(b.recordByID, old.ID)
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
	old, had := b.users[name]
	if _, seen := s.users[name]; !seen {
		if had {
			s.users[name] = &old
		} else {
			s.users[name] = nil
		}
	}
	now := time.Now()
	if had {
		u.ID, u.Created = old.ID, old.Created
	} else {
		u.ID = b.takeID(&b.nextUserID)
		u.Created = now
	}
	u.Updated = now
	b.users[name] = u
	b.userByID[u.ID] = name
}

// setGroup writes a Group's membership, its allow list, creating the Group's
// record owned by owner when there is none: a Group is an ordinary record
// (docs/constitution.md#-group). Caller holds b.mu.
func (b *Bus) setGroup(name, owner string, members []string) {
	r, had := b.records[name]
	if !had {
		r = protocol.Record{Name: name, Kind: protocol.KindGroup, Owner: owner}
	}
	r.Allow = append([]string{}, members...)
	r.At = time.Now()
	b.setRecord(name, r)
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
	if b.idsExhausted {
		b.idsExhausted = false
		b.rollback(s)
		return ErrExhausted
	}
	if b.store != nil {
		if err := b.store.Commit(b.change(s)); err != nil {
			b.rollback(s)
			// A store that will not take a write is loss of authoritative
			// answering, not a caller's mistake (docs/constitution.md#errors-and-alerts).
			b.report(ports.Error, "a management write was not committed and nothing changed: %s", err)
			return fmt.Errorf("persist administrative state: %w", err)
		}
		for name := range s.inboxes {
			delete(b.flushed, name)
		}
	}
	b.publishCredentials(s.creds)
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
	for name := range s.inboxes {
		if _, back := b.inboxes[name]; !back {
			c.DropQueues = append(c.DropQueues, name)
		}
	}
	// A record born in this write owns no stored queue. One may still be on
	// disk under its name, left by an ignored record the name used to belong
	// to, and the next start would hand that one's messages to the new owner.
	// Its own in-memory inbox, if it has one yet, is written by the next flush.
	for name, before := range s.records {
		_, now := b.records[name]
		if _, dropping := s.inboxes[name]; before == nil && now && !dropping {
			c.DropQueues = append(c.DropQueues, name)
		}
	}
	if len(s.creds) > 0 {
		c.Credentials = s.creds
	}
	if s.owner != nil {
		owner := b.admin
		c.Owner = &owner
	}
	if s.accountsOn {
		c.Accounts = cloneAccounts(b.accounts)
	}
	if s.idsOn {
		nextRecord, nextUser := b.nextRecordID, b.nextUserID
		c.NextRecordID, c.NextUserID = &nextRecord, &nextUser
	}
	return c
}

// rollback restores every staged entity to its value before the write.
func (b *Bus) rollback(s *staged) {
	for name, old := range s.records {
		if now, has := b.records[name]; has {
			delete(b.recordByID, now.ID)
		}
		if old == nil {
			delete(b.records, name)
		} else {
			b.records[name] = *old
			b.recordByID[old.ID] = name
		}
	}
	for name, old := range s.users {
		if now, has := b.users[name]; has {
			delete(b.userByID, now.ID)
		}
		if old == nil {
			delete(b.users, name)
		} else {
			b.users[name] = *old
			b.userByID[old.ID] = name
		}
	}
	if s.idsOn {
		b.nextRecordID, b.nextUserID = s.nextRecordID, s.nextUserID
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

// Journal binds where the bus reports warnings, errors and alerts.
func (b *Bus) Journal(j ports.Journal) {
	b.mu.Lock()
	defer b.unlock()
	b.journal = j
}

// report writes one warning, error or alert. Caller may hold b.mu: the journal
// never calls back into the bus.
func (b *Bus) report(sev ports.Severity, format string, args ...any) {
	if b.journal != nil {
		b.journal.Report(sev, fmt.Sprintf(format, args...))
	}
}
