package core

import (
	"fmt"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Unregister removes an idle address, not its principal or credential.
// See docs/01-identity.md#unregistering.
func (b *Bus) Unregister(name, caller string) error {
	n, err := canon(name)
	if err != nil {
		return err
	}
	who, err := canon(caller)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r, known := b.records[n]
	if !known {
		return fmt.Errorf("%w: %s", ErrUnknown, n)
	}
	if !b.manages(who, r) {
		return fmt.Errorf("%w: %s", ErrNotOwner, n)
	}
	if in := b.inboxes[n]; in != nil {
		b.prune(in, time.Now())
		if len(in.queue) != 0 || len(in.waiters) != 0 {
			return fmt.Errorf("%w: %s has %d queued messages and %d waiting readers; drain its queue and stop its readers first", ErrBusy, n, len(in.queue), len(in.waiters))
		}
	}
	// Tokens still authenticate this name. Preserve who may reclaim it.
	b.retired[n] = r.Owner
	delete(b.records, n)
	delete(b.inboxes, n)
	for name, topic := range b.records {
		if len(topic.Subs) != 0 {
			topic.Subs = drop1(topic.Subs, n)
			b.records[name] = topic
		}
	}
	return nil
}

// recordOrReservation is used for ownership, never discovery or routing.
// Caller holds b.mu. A reclaimed address starts with bare-registration defaults.
func (b *Bus) recordOrReservation(name string) (protocol.Record, bool) {
	if r, ok := b.records[name]; ok {
		return r, true
	}
	owner, ok := b.retired[name]
	return protocol.Record{Name: name, Owner: owner, Kind: "generic", Full: protocol.OverflowStrict}, ok
}
