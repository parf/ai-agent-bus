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
	defer b.mu.Unlock()
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

// Persistence binds the existing snapshot port before serving requests.
// Unbound buses are intentionally in-memory (unit tests and embedded use).
func (b *Bus) Persistence(d ports.Dump) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dump = d
}

// Checkpoint serializes capture AND replacement with administrative mutations.
// Taking a snapshot first and locking only Save would let an older periodic
// write restore authority after a newer restriction had already been acknowledged.
func (b *Bus) Checkpoint(clean bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.checkpoint(clean)
}

// checkpoint runs under the operation's hold. On failure there is no success
// acknowledgement; memory may already contain the change. Never imply rollback.
func (b *Bus) checkpoint(clean bool) error {
	if b.dump == nil {
		return nil
	}
	s := b.snapshot()
	s.Clean = clean
	if err := b.dump.Save(s); err != nil {
		return fmt.Errorf("persist administrative state: %w", err)
	}
	return nil
}

// Restore puts a snapshot back. Expiry is not re-checked here: a message
// carries the moment it stops being worth delivering, and Consume is the one
// place that decides — a reloaded queue is no different from one that sat
// through a quiet hour. Uptime is not restored: it is this run's.
func (b *Bus) Restore(s ports.Snapshot) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s = migrateAdministrators(s)
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
	for _, r := range s.Records {
		// A snapshot may have been written by another version or supplied by
		// an embedding caller. Live fields belong to this process and its
		// current inboxes, never to durable registry state.
		clearLiveRecord(&r)
		b.records[r.Name] = r
	}
	for _, q := range s.Queues {
		in := b.ensure(q.Name)
		in.in, in.out = q.In, q.Out
		in.dropped, in.expired = q.Dropped, q.Expired
		in.queue = append(in.queue, q.Messages...)
	}
}
