package core

import (
	"fmt"
	"strings"
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
	// Removing the record that makes a name a principal, while that name still
	// owns others, leaves every one of them owned by somebody the daemon no
	// longer knows: the wreckage the deletion rule exists for
	// (docs/01-identity.md#when-the-owner-is-gone), made by an ordinary call.
	// Refused like a queue that is not empty, and for the same reason — there
	// is somebody here to tell, and what to do about it is theirs to choose.
	//
	// What matters is what removing it costs, not who owned it: a record is
	// the whole of a name's standing unless it also has a profile, so taking
	// it leaves that name known to nobody however the record was owned. A
	// registered user keeps its services, because the user is still there.
	if _, person := b.users[n]; !person {
		if owned := b.ownedBy(n); len(owned) != 0 {
			return fmt.Errorf("%w: %s still owns %s; remove or hand those over first", ErrBusy, n, strings.Join(owned, ", "))
		}
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
