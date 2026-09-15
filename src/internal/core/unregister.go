package core

import (
	"fmt"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Unregister removes an idle address and everything that kept it reachable.
// A removed name is not reserved and not reclaimable: the address is gone and
// so is the credential the face drops with it. Protecting a removed name is
// [R1.2 work](../../Plans/R1.2/README.md#removed-names), deliberately not
// MVP's. See docs/01-identity.md#unregistering.
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

// record is used for ownership, never discovery or routing. Caller holds b.mu.
// A name with no record is owned by nobody: nothing survives unregistering to
// say who held it.
func (b *Bus) record(name string) (protocol.Record, bool) {
	r, ok := b.records[name]
	return r, ok
}
