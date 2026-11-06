package core

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Snapshot is the bus's memory at this moment, for whatever writes it down.
// Records travel with the queues because an inbox without its record is a
// backlog nobody may read. See docs/04-messaging.md#durability.
func (b *Bus) Snapshot() ports.Snapshot {
	b.mu.Lock()
	defer b.unlock()
	return b.snapshot()
}

// snapshot is called with b.mu held.
func (b *Bus) snapshot() ports.Snapshot {
	s := ports.Snapshot{
		OwnerEstablished:    b.admin != "",
		Owner:               b.admin,
		AccountsEstablished: true,
		At:                  time.Now(),
		NextRecordID:        b.nextRecordID,
		NextUserID:          b.nextUserID,
	}
	for account, principal := range b.accounts {
		s.Accounts = append(s.Accounts, protocol.AccountMapping{Account: account, Principal: principal})
	}
	sort.Slice(s.Accounts, func(i, j int) bool { return s.Accounts[i].Account < s.Accounts[j].Account })
	for _, user := range b.users {
		s.Users = append(s.Users, user)
	}
	for _, r := range b.records {
		s.Records = append(s.Records, r)
	}
	for name, in := range b.inboxes {
		// A blocked reader is not state: its connection died with the
		// process, so it is not saved and not counted.
		if in.in == 0 && in.out == 0 && len(in.queue) == 0 {
			continue
		}
		s.Queues = append(s.Queues, ports.Queue{
			Name: name, In: in.in, Out: in.out,
			Dropped: in.dropped, Expired: in.expired,
			Messages: append([]protocol.Envelope(nil), in.queue...),
			Activity: in.act.Save(in.totals()),
		})
	}
	s.Activity = b.node.Save(b.refusedTotal())
	return s
}

// Persistence binds the store before serving requests. Unbound buses are
// intentionally in-memory (unit tests and embedded use). A bound store must
// expose the complete event persistence family; startup must handle the error
// before serving. Bind before first use: rebinding does not rewire an engine
// or subscription cache that was already initialized.
func (b *Bus) Persistence(st ports.Store) error {
	b.mu.Lock()
	defer b.unlock()
	if err := validateEventStore(st); err != nil {
		return err
	}
	b.store = st
	return nil
}

// queueMark is what a queue's counters were when it was last saved. Every
// change to a queue's contents moves one of them — a delivery moves in, a
// read out, a loss dropped or expired — and every change to its activity one
// of those or refused, so an unchanged mark is an unchanged queue. A ring
// that only moved on through quiet slots needs no save: a restore reads the
// time since as zero.
type queueMark struct{ in, out, dropped, expired, refused int }

// FlushQueues writes the queues whose state moved since the last flush, as one
// batch: every minute and at a graceful stop, never once per message. clean
// says this is the graceful stop's, which is how the next start tells a clean
// stop from a death (docs/04-messaging.md#durability).
//
// The copies are taken under b.mu and written without it: a database write
// never holds the bus. A commit that drops or replaces one of those queues
// meanwhile is written first, so the flush then writes that queue again as it
// is now, or drops it, rather than leave its older copy on disk.
func (b *Bus) FlushQueues(clean bool) error {
	b.flushing.Lock()
	defer b.flushing.Unlock()
	b.mu.Lock()
	st := b.store
	if st == nil {
		b.unlock()
		return nil
	}
	var names []string
	for name := range b.inboxes {
		names = append(names, name)
	}
	qs, marks := b.dirtyQueues(names)
	var node []byte
	if b.activityStore() == nil {
		node = b.node.Save(b.refusedTotal())
	}
	b.inFlight, b.overtaken = map[string]bool{}, map[string]bool{}
	for name := range marks {
		b.inFlight[name] = true
	}
	b.unlock()

	err := st.SaveQueues(qs, node, clean)

	b.mu.Lock()
	defer b.unlock()
	overtaken := b.overtaken
	b.inFlight, b.overtaken = nil, nil
	if err != nil {
		return fmt.Errorf("persist queues: %w", err)
	}
	for name, m := range marks {
		if !overtaken[name] {
			b.flushed[name] = m
		}
	}
	if len(overtaken) == 0 {
		return nil
	}
	var again, drop []string
	for name := range overtaken {
		if _, known := b.records[name]; known && b.inboxes[name] != nil {
			again = append(again, name)
		} else {
			drop = append(drop, name)
		}
	}
	if len(drop) > 0 {
		if err := b.store.Commit(ports.Change{DropQueues: drop}); err != nil {
			return fmt.Errorf("persist queues: %w", err)
		}
	}
	qs, marks = b.dirtyQueues(again)
	if err := b.store.SaveQueues(qs, nil, clean); err != nil {
		return fmt.Errorf("persist queues: %w", err)
	}
	for name, m := range marks {
		b.flushed[name] = m
	}
	return nil
}

// dirtyQueues copies the named queues that moved since their last flush.
// Called with b.mu held.
func (b *Bus) dirtyQueues(names []string) ([]ports.Queue, map[string]queueMark) {
	var qs []ports.Queue
	marks := map[string]queueMark{}
	for _, name := range names {
		in := b.inboxes[name]
		// A queue under a name with no record would be a backlog nobody may
		// read; it is not saved.
		if _, known := b.records[name]; !known || in == nil {
			continue
		}
		m := queueMark{in.in, in.out, in.dropped, in.expired, in.refused}
		if old, saved := b.flushed[name]; saved && old == m {
			continue
		}
		qs = append(qs, ports.Queue{
			Name: name, In: in.in, Out: in.out,
			Dropped: in.dropped, Expired: in.expired,
			Messages: append([]protocol.Envelope(nil), in.queue...),
			Activity: b.queueActivity(in),
		})
		marks[name] = m
	}
	return qs, marks
}

// overtake notes that a commit changed a queue a flush is writing. Called with
// b.mu held.
func (b *Bus) overtake(name string) {
	if b.inFlight[name] {
		b.overtaken[name] = true
	}
}

// unlock releases b.mu. A management write that reached here without
// committing failed on the way — validation or otherwise — and is put back,
// so a refused write publishes nothing (docs/constitution.md#persistence-and-loading).
func (b *Bus) unlock() {
	if b.staging != nil {
		b.discard()
	}
	b.mu.Unlock()
}

// Restore puts a snapshot back. Expiry is not re-checked here: a message
// carries the moment it stops being worth delivering, and Consume is the one
// place that decides — a reloaded queue is no different from one that sat
// through a quiet hour. Uptime is not restored: it is this run's. It must not
// run while a queue flush is in flight: a flush's corrections cover commits,
// not a restore (the daemon restores once, before the first flush).
func (b *Bus) Restore(s ports.Snapshot) error {
	if err := ValidateSnapshot(s); err != nil {
		b.mu.Lock()
		b.ownerRestoreErr = err
		b.accountRestoreErr = err
		b.mu.Unlock()
		return err
	}
	b.mu.Lock()
	defer b.unlock()
	if b.events != nil {
		nextIDs := map[string]uint32{}
		for _, next := range s.Records {
			nextIDs[next.Name] = next.ID
		}
		for name, r := range b.records {
			if nextIDs[name] != r.ID {
				b.events.Forget(r.ID)
				b.forgetSubs(r.ID)
			}
		}
	}

	b.ownerRestored, b.accountsRestored = s.OwnerEstablished, s.AccountsEstablished
	b.ownerRestoreErr, b.accountRestoreErr = nil, nil
	if s.OwnerEstablished {
		b.admin = s.Owner
	}
	b.accounts = map[string]string{}
	for _, mapping := range s.Accounts {
		b.accounts[mapping.Account] = mapping.Principal
	}
	// A database that has never been written has had no run to lose.
	b.unclean = !s.Clean && !s.At.IsZero()
	if s.NextRecordID > b.nextRecordID {
		b.nextRecordID = s.NextRecordID
	}
	if s.NextUserID > b.nextUserID {
		b.nextUserID = s.NextUserID
	}
	for _, user := range s.Users {
		// Derived answers belong to this process and its current visitor.
		clearDerivedUser(&user)
		b.users[user.Name] = user
	}
	// Restore loads; it commits nothing and repairs nothing.
	b.staging = nil
	for _, r := range s.Records {
		// Live fields are derived from this process, never durable state.
		clearLiveRecord(&r)
		b.records[r.Name] = r
	}
	b.indexIDs()
	for name, r := range b.records {
		r.ParsedAllow, _ = protocol.CompileACL(r.Allow)
		b.records[name] = r
	}
	for _, q := range s.Queues {
		in := b.ensure(q.Name)
		in.in, in.out = q.In, q.Out
		in.dropped, in.expired = q.Dropped, q.Expired
		in.queue = append(in.queue, q.Messages...)
		var readable bool
		in.act, readable = b.restoreActivity(q.Name, q.Activity, in.totals())
		if readable {
			b.flushed[q.Name] = queueMark{in.in, in.out, in.dropped, in.expired, in.refused}
		}
	}
	b.node, _ = b.restoreActivity("the node", s.Activity, b.refusedTotal())
	for _, id := range s.EventIDIssues {
		name := b.recordByID[id]
		if name == "" {
			b.report(ports.Error, "invalid orphan agent-event counter")
		} else {
			b.report(ports.Error, "invalid agent-event counter for %s", name)
		}
	}

	return nil
}

// indexIDs rebuilds the ID indexes from the loaded entities and moves the
// high-water marks past every validated ID in use.
// Caller holds b.mu.
func (b *Bus) indexIDs() {
	b.recordByID, b.userByID = map[uint32]string{}, map[uint32]string{}
	names := make([]string, 0, len(b.records))
	for name := range b.records {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		r := b.records[name]
		if r.ID >= b.nextRecordID {
			b.nextRecordID = r.ID + 1
		}
	}
	for _, name := range names {
		r := b.records[name]

		b.recordByID[r.ID] = name
	}
	users := make([]string, 0, len(b.users))
	for name := range b.users {
		users = append(users, name)
	}
	sort.Strings(users)
	for _, name := range users {
		if u := b.users[name]; u.ID >= b.nextUserID {
			b.nextUserID = u.ID + 1
		}
	}
	for _, name := range users {
		u := b.users[name]

		b.userByID[u.ID] = name
	}
}

// RecordName and UserName answer an internal ID with the name it belongs to,
// from the index rebuilt at load.
func (b *Bus) RecordName(id uint32) (string, bool) {
	b.mu.Lock()
	defer b.unlock()
	name, ok := b.recordByID[id]
	return name, ok
}

func (b *Bus) UserName(id uint32) (string, bool) {
	b.mu.Lock()
	defer b.unlock()
	name, ok := b.userByID[id]
	return name, ok
}

// validateRestored checks durable representation without rewriting it.
func validateRestored(r protocol.Record) error {
	tags, err := protocol.NormalizeTags(r.Tags)
	if err != nil {
		return err
	}
	if !slices.Equal(tags, r.Tags) {
		return fmt.Errorf("stored tags are not canonical labels")
	}
	if holdsPrivate(r.Kind) {
		if err := configSize(r.Config); err != nil {
			return err
		}
	}
	return validateKind(r)
}

// incorrect says why a stored record could not have been written by this
// version, or "" when it could. Caller holds b.mu.
func (b *Bus) incorrect(r protocol.Record) string {
	hasAssignments := false
	for _, raw := range r.Allow {
		_, roles, err := protocol.ParseACLTerm(raw)
		if err != nil {
			return err.Error()
		}
		hasAssignments = hasAssignments || len(roles) > 0
		if len(roles) > 0 && (r.Kind == protocol.KindGroup || r.Kind == protocol.KindUser) {
			return "group membership and user records take no assigned roles"
		}
	}
	if hasAssignments {
		if _, err := b.normalizeAllow(r.Allow, r); err != nil {
			return err.Error()
		}
	}
	if err := validateRestored(r); err != nil {
		return err.Error()
	}
	u, user := b.users[r.Owner]
	if !user {
		return "its owner " + r.Owner + " is not a user"
	}
	// Owned by a User of that name who is not this one: the one who owned it
	// vanished and the name was given again.
	if r.OwnerID != u.ID {
		return fmt.Sprintf("its owner %s is user %d, not the user %d that owned it", r.Owner, u.ID, r.OwnerID)
	}
	if r.Kind == protocol.KindUser && r.Owner != r.Name {
		return "a user record belongs to its own user"
	}
	if err := b.validatePersonal(r); err != nil {
		return err.Error()
	}
	// A nested group nothing holds would pass its grant to whoever created
	// it, which no write of this version allows (manage.go groupMember).
	if r.Kind == protocol.KindGroup {
		for _, m := range r.Allow {
			if g, ok := b.records[m]; groupName(m) && (!ok || g.Kind != protocol.KindGroup) {
				return "its member " + m + " is no group"
			}
		}
	}
	return ""
}

// queueActivity is the ring a queue row carries: none once the store keeps
// days, which the ten-minute boundary writes instead of every flush.
func (b *Bus) queueActivity(in *inbox) []byte {
	if b.activityStore() != nil {
		return nil
	}
	return in.act.Save(in.totals())
}
