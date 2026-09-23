package core

import (
	"fmt"
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
		Groups:              map[string][]string{},
	}
	for account, principal := range b.accounts {
		s.Accounts = append(s.Accounts, protocol.AccountMapping{Account: account, Principal: principal})
	}
	sort.Slice(s.Accounts, func(i, j int) bool { return s.Accounts[i].Account < s.Accounts[j].Account })
	for _, user := range b.users {
		s.Users = append(s.Users, user)
	}
	for name, members := range b.groups {
		s.Groups[name] = append([]string{}, members...)
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
		})
	}
	return s
}

// Persistence binds the store before serving requests. Unbound buses are
// intentionally in-memory (unit tests and embedded use).
func (b *Bus) Persistence(st ports.Store) {
	b.mu.Lock()
	defer b.unlock()
	b.store = st
}

// queueMark is what a queue's counters were when it was last saved. Every
// change to a queue's contents moves one of them — a delivery moves in, a
// read out, a loss dropped or expired — so an unchanged mark is an unchanged
// queue.
type queueMark struct{ in, out, dropped, expired int }

// FlushQueues writes the queues whose state moved since the last flush, as one
// batch: every minute and at a graceful stop, never once per message. clean
// says this is the graceful stop's, which is how the next start tells a clean
// stop from a death (docs/04-messaging.md#durability).
func (b *Bus) FlushQueues(clean bool) error {
	b.mu.Lock()
	defer b.unlock()
	if b.store == nil {
		return nil
	}
	var qs []ports.Queue
	marks := map[string]queueMark{}
	for name, in := range b.inboxes {
		// A queue under a name with no record would be a backlog nobody may
		// read; it is not saved.
		if _, known := b.records[name]; !known {
			continue
		}
		m := queueMark{in.in, in.out, in.dropped, in.expired}
		if old, saved := b.flushed[name]; saved && old == m {
			continue
		}
		qs = append(qs, ports.Queue{
			Name: name, In: in.in, Out: in.out,
			Dropped: in.dropped, Expired: in.expired,
			Messages: append([]protocol.Envelope(nil), in.queue...),
		})
		marks[name] = m
	}
	if err := b.store.SaveQueues(qs, clean); err != nil {
		return fmt.Errorf("persist queues: %w", err)
	}
	for name, m := range marks {
		b.flushed[name] = m
	}
	return nil
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
// through a quiet hour. Uptime is not restored: it is this run's.
func (b *Bus) Restore(s ports.Snapshot) {
	b.mu.Lock()
	defer b.unlock()
	b.ownerRestored = s.OwnerEstablished || s.Owner != ""
	b.ownerRestoreErr = nil
	if s.Owner != "" && !s.OwnerEstablished {
		b.admin = ""
		b.ownerRestoreErr = fmt.Errorf("snapshot has daemon owner without establishment marker")
	}
	if s.OwnerEstablished {
		b.admin = ""
		owner, err := canon(s.Owner)
		if err != nil {
			b.ownerRestoreErr = fmt.Errorf("snapshot daemon owner: %w", err)
		} else {
			b.admin = owner
		}
	}
	b.accountsRestored = s.AccountsEstablished || len(s.Accounts) > 0
	b.accountRestoreErr = nil
	b.accounts = map[string]string{}
	if len(s.Accounts) > 0 && !s.AccountsEstablished {
		b.accountRestoreErr = fmt.Errorf("snapshot has local account mappings without establishment marker")
	}
	if s.AccountsEstablished {
		for _, mapping := range s.Accounts {
			principal, err := canon(mapping.Principal)
			if mapping.Account == "" || err != nil {
				b.accountRestoreErr = fmt.Errorf("snapshot local account mapping %q=%q is invalid", mapping.Account, mapping.Principal)
				break
			}
			if _, duplicate := b.accounts[mapping.Account]; duplicate {
				b.accountRestoreErr = fmt.Errorf("snapshot repeats local account %q", mapping.Account)
				break
			}
			b.accounts[mapping.Account] = principal
		}
	}
	b.unclean = !s.Clean
	for _, user := range s.Users {
		// A snapshot may come from another version or an embedding caller.
		// Derived answers belong to this process and its current visitor.
		clearDerivedUser(&user)
		b.users[user.Name] = user
	}
	for name, members := range s.Groups {
		b.groups[name] = append([]string{}, members...)
	}
	if b.ownerRestoreErr == nil {
		if _, stored := b.groups[OwnerGroup]; stored {
			b.ownerRestoreErr = fmt.Errorf("snapshot contains runtime ACL term %s as a stored group", OwnerGroup)
		}
	}
	if b.ownerRestoreErr == nil {
		for _, member := range b.groups[AdministratorsGroup] {
			if groupName(member) {
				b.ownerRestoreErr = fmt.Errorf("snapshot %s contains nested group %s; administrative membership is direct-only", AdministratorsGroup, member)
				break
			}
		}
	}
	if s.OwnerEstablished && b.ownerRestoreErr == nil {
		if _, known := b.users[b.admin]; !known {
			b.ownerRestoreErr = fmt.Errorf("snapshot daemon owner %s is not a registered user", b.admin)
		} else if err := b.acting(b.admin); err != nil {
			b.ownerRestoreErr = fmt.Errorf("snapshot daemon owner %s is not active: %w", b.admin, err)
		} else if !b.member(b.admin, AdministratorsGroup) {
			b.groups[AdministratorsGroup] = append(b.groups[AdministratorsGroup], b.admin)
			sort.Strings(b.groups[AdministratorsGroup])
		}
	}
	// A snapshot written before administrators had to be users can hold one who
	// is not; the invariant is restored rather than trusted. This happens after
	// the durable-owner check so it cannot manufacture a missing owner profile.
	b.administratorsAreUsers()
	// Restore loads; it commits nothing. What administratorsAreUsers derived
	// stays in memory as the load's own repair rather than a staged write.
	b.staging = nil
	for _, r := range s.Records {
		// A snapshot may have been written by another version or supplied by
		// an embedding caller. Live fields belong to this process and its
		// current inboxes, never to durable registry state.
		clearLiveRecord(&r)
		// A record this daemon would not accept is not silently kept: it would
		// be state the daemon cannot describe, and describing a record is the
		// whole reason the set is closed. Nothing before 1.1 carries a
		// compatibility obligation, so this refuses rather than converts, and
		// it asks the same question registration asks rather than a weaker one
		// — an unknown kind and a service with no address are equally records
		// this version cannot serve.
		// See docs/03-records.md#five-record-kinds.
		if b.recordRestoreErr == nil {
			if err := validateKind(r); err != nil {
				b.recordRestoreErr = fmt.Errorf("snapshot record %s cannot be restored: %w", r.Name, err)
			}
		}
		b.records[r.Name] = r
	}
	// Personal validation asks about OTHER records, so it runs once they are
	// all here: a snapshot does not promise that a Personal record's ACL is
	// listed after the records it names.
	for _, r := range s.Records {
		if b.recordRestoreErr != nil {
			break
		}
		if err := b.validatePersonal(b.records[r.Name]); err != nil {
			b.recordRestoreErr = fmt.Errorf("snapshot record %s cannot be restored: %w", r.Name, err)
		}
	}
	for _, q := range s.Queues {
		// A queue under a service name is a snapshot this version could not
		// have written, and restoring it would leave messages nothing can
		// ever read. Refused rather than dropped, for the same reason an
		// unknown kind is. See docs/03-records.md#five-record-kinds.
		if r, known := b.records[q.Name]; known && !onBus(r) && b.recordRestoreErr == nil {
			b.recordRestoreErr = fmt.Errorf("snapshot queue %s cannot be restored: %w: a service has no queue here", q.Name, ErrKind)
		}
		in := b.ensure(q.Name)
		in.in, in.out = q.In, q.Out
		in.dropped, in.expired = q.Dropped, q.Expired
		in.queue = append(in.queue, q.Messages...)
		b.flushed[q.Name] = queueMark{in.in, in.out, in.dropped, in.expired}
	}
}
