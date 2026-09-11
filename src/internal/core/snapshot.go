package core

import (
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
	s := ports.Snapshot{At: time.Now(), Dropped: b.dropped, Expired: b.expired}
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
			Messages: append([]protocol.Envelope(nil), in.queue...),
		})
	}
	return s
}

// Restore puts a snapshot back. Expiry is not re-checked here: a message
// carries the moment it stops being worth delivering, and Consume is the one
// place that decides — a reloaded queue is no different from one that sat
// through a quiet hour. Uptime is not restored: it is this run's.
func (b *Bus) Restore(s ports.Snapshot) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dropped, b.expired = s.Dropped, s.Expired
	for _, r := range s.Records {
		b.records[r.Name] = r
	}
	for _, q := range s.Queues {
		in := b.ensure(q.Name)
		in.in, in.out = q.In, q.Out
		in.queue = append(in.queue, q.Messages...)
	}
}
