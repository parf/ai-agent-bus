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
// MVP's. See docs/01-identity-and-roles.md#unregistering.
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
	defer b.unlock()
	// Ahead of the record, so that removing something is refused with who you
	// are rather than with whose it is.
	if err := b.acting(who); err != nil {
		return err
	}
	r, known := b.entity(n)
	if !known {
		return fmt.Errorf("%w: %s", ErrUnknown, n)
	}
	if !b.manages(who, r) {
		return fmt.Errorf("%w: %s", ErrNotOwner, n)
	}
	// A Group is retired by emptying it, never removed: its name is what
	// every list naming it resolves (docs/01-identity-and-roles.md#groups).
	if r.Kind == protocol.KindGroup {
		return fmt.Errorf("%w: %s is a group; retire it by emptying it", ErrNoRemoval, n)
	}
	// A User's own record goes with the User, and a User is never removed
	// (docs/constitution.md#-user).
	if r.Kind == protocol.KindUser {
		return fmt.Errorf("%w: %s is a user's own record, and a user is never removed", ErrBusy, n)
	}
	// Only a User owns records, and a User's own record is never removed, so
	// nothing is ever owned by the name going here.
	if in := b.inboxes[n]; in != nil {
		b.prune(in, time.Now())
		if len(in.queue) != 0 || len(in.waiters) != 0 {
			return fmt.Errorf("%w: %s has %d queued messages and %d waiting readers; drain its queue and stop its readers first", ErrBusy, n, len(in.queue), len(in.waiters))
		}
	}
	// The credential goes with the address, in the removal's own commit: a
	// record deleted and a credential kept is a name answering for nobody, and
	// the old bytes must not answer for whoever registers the name next.
	// See docs/02-access.md#token-lifetime.
	b.stageCredential(n, nil)
	// A local account mapped to the name goes with it, or whoever registered
	// the name next would inherit that account's socket.
	next, dropped := cloneAccounts(b.accounts), false
	for account, principal := range next {
		if principal == n {
			delete(next, account)
			dropped = true
		}
	}
	if dropped {
		b.setAccounts(next)
	}
	b.forgetName(n)
	// The name has stopped being a principal, so its reads of other inboxes
	// have stopped being reads anybody is entitled to. Its own inbox is gone;
	// these are the waits it left elsewhere.
	b.recheckReaders()
	return b.commit()
}

// forgetName takes every trace of a name that is no longer a principal: the
// record, the inbox, and every reference to it — ACL, Maintainer, Group
// member and Deliver-To — in the same write. A freed name is registrable by
// anybody (docs/01-identity-and-roles.md#unregistering), so a reference left
// behind is not a dangling row: it is inherited by whoever takes the name
// next. Caller holds b.mu.
func (b *Bus) forgetName(name string) {
	b.dropRecord(name)
	b.dropInbox(name)
	for other, r := range b.records {
		allow, maint, subs := drop1(r.Allow, name), drop1(r.Maintainers, name), drop1(r.Subs, name)
		if len(allow) != len(r.Allow) || len(maint) != len(r.Maintainers) || len(subs) != len(r.Subs) {
			r.Allow, r.Maintainers, r.Subs = allow, maint, subs
			b.setRecord(other, r)
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
