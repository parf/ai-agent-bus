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
// MVP's. See docs/01-identity-and-roles.md#unregistering.
func (b *Bus) Unregister(name, caller string) error {
	return b.UnregisterAnd(name, caller, nil)
}

// UnregisterAnd removes the address and drops its credential together, under
// one hold.
//
// Done in two calls, the gap between them was the whole of the problem: the
// record went, the lock was released, and the name was decided to be a service
// rather than a person — and its credential dropped — on facts that were no
// longer true. In that gap the name could be registered again by somebody
// else, whose credential was then the one forgotten.
//
// forget runs before anything is deleted and its failure abandons the removal,
// because the order that can strand something is the other one: a record
// deleted and a credential kept is a name answering for nobody, which is the
// state all of this exists to prevent. A credential dropped for a record that
// then stays is recoverable — ask for another.
//
// forget must only touch the credential store, never call back into Bus. Same
// shape and same hold as IssueFor and RemoveOwnerless.
func (b *Bus) UnregisterAnd(name, caller string, forget func(string) error) error {
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
	// Ahead of the record, so that removing something is refused with who you
	// are rather than with whose it is.
	if err := b.acting(who); err != nil {
		return err
	}
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
	// (docs/01-identity-and-roles.md#orphaned-records), made by an ordinary call.
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
	// The credential goes with the address. A person's own identity is the
	// exception, and not for the same reason: their credential is how they
	// call at all, and unregistering a record must not log them out. Decided
	// here, on the same facts the removal is decided on.
	// See docs/01-identity-and-roles.md#unregistering.
	if _, person := b.users[n]; !person && forget != nil {
		if err := forget(n); err != nil {
			return err
		}
	}
	// A person keeps their profile, their credential and their standing when
	// a record of theirs is removed; everything below is for a name that has
	// stopped being a principal at all.
	if _, person := b.users[n]; person {
		delete(b.records, n)
		delete(b.inboxes, n)
		for name, topic := range b.records {
			if len(topic.Subs) != 0 {
				topic.Subs = drop1(topic.Subs, n)
				b.records[name] = topic
			}
		}
	} else {
		b.forgetName(n)
	}
	// The name has stopped being a principal, so its reads of other inboxes
	// have stopped being reads anybody is entitled to. Its own inbox is gone;
	// these are the waits it left elsewhere.
	b.recheckReaders()
	return nil
}

// forgetName takes every trace of a name that is no longer a principal: the
// record, the inbox, the subscriptions it held, and the groups it was in.
// Shared with the startup purge (orphans.go) so the two cannot disagree about
// what a name leaves behind. Caller holds b.mu.
func (b *Bus) forgetName(name string) {
	delete(b.records, name)
	delete(b.inboxes, name)
	for other, topic := range b.records {
		if len(topic.Subs) != 0 {
			topic.Subs = drop1(topic.Subs, name)
			b.records[other] = topic
		}
	}
	// **Group membership goes with the name.** A freed name is reclaimable by
	// anybody (docs/01-identity-and-roles.md#unregistering), so a membership left
	// behind is not a dangling row — it is inherited. Whoever registers the
	// name next is in every group the old one was, and reaches every record
	// those groups allow.
	for group, members := range b.groups {
		if next := drop1(members, name); len(next) != len(members) {
			b.groups[group] = next
		}
	}
}

// record is used for ownership, never discovery or routing. Caller holds b.mu.
// A name with no record is owned by nobody: nothing survives unregistering to
// say who held it.
func (b *Bus) record(name string) (protocol.Record, bool) {
	r, ok := b.records[name]
	return r, ok
}
