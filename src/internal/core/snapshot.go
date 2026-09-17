package core

import (
	"fmt"
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
	s := ports.Snapshot{At: time.Now(), Groups: map[string][]string{}}
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
	b.unclean = !s.Clean
	for _, user := range s.Users {
		b.users[user.Name] = user
	}
	for name, members := range s.Groups {
		b.groups[name] = append([]string{}, members...)
	}
	// A snapshot written before administrators had to be users can hold one who
	// is not; the invariant is restored rather than trusted.
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
