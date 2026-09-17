package core

import (
	"fmt"
	"sort"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Orphans deletes every record whose owner the daemon knows nothing about,
// and returns what it took. A service whose owner is not known is not a state
// to recover from: it is wreckage from an older store, or from something that
// went wrong, and it goes at once with everything that hung on it
// (docs/01-identity-and-roles.md#orphaned-records).
//
// **Known is a profile or a record, not a profile.** A self-owned record with
// no user behind it is a principal the daemon supports: it authenticates, it
// may be handed a record by transfer, and it may register records of its own.
// Deleting it because no *user* stands at the end of the chain would undo
// operations the daemon had just accepted, so ownership is asked about one
// step, not walked to a person.
//
// **It runs to a fixed point, because deletions make orphans.** A owns B and B
// owns C, all of them records: taking A away is what makes B unknown, and
// taking B away is what makes C unknown. One pass leaves C live, holding a
// queue nobody may read, until some later restart happens to notice.
//
// A cycle survives, and that is the same rule rather than an exception: each
// member's owner has a record of its own, so each is known. No live call can
// build one — transfer demands a self-owned principal and re-registration
// keeps the owner it had — so a cycle in the store came from outside the
// daemon, and inventing a rule for it here would be inventing the rule too.
//
// Caller does not hold b.mu.
func (b *Bus) Orphans() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	gone := []string{}
	for {
		round := []string{}
		for name, r := range b.records {
			if b.wreckage(r) {
				round = append(round, name)
			}
		}
		if len(round) == 0 {
			break
		}
		// Sorted so that a log, and a test, read the same way twice.
		sort.Strings(round)
		for _, name := range round {
			b.orphan(name)
		}
		gone = append(gone, round...)
	}
	// The names that went have stopped being principals, so their reads of
	// inboxes that SURVIVED have stopped being reads anybody is entitled to.
	// Their own inboxes are gone; these are the waits they left elsewhere, and
	// nothing else collects them — the same call UnregisterAnd ends with, for
	// the same reason.
	if len(gone) != 0 {
		b.recheckReaders()
	}
	return gone
}

// wreckage says a record answers for nobody. Caller holds b.mu.
//
// **A registered user's own record is never wreckage, whoever owns it.** The
// rule above rests on there being no principal behind the name — and a person
// is one. Their profile is their standing, they may manage their own record
// (`manages` allows the name itself), and messages addressed to them reach
// somebody. This is the exemption the credential sweep makes too, for the same
// one reason: a user is never deleted (docs/01-identity-and-roles.md#user-states).
// Without it the sweep took a person's inbox, their group membership and — by
// way of the name it handed back — their credential.
func (b *Bus) wreckage(r protocol.Record) bool {
	if b.identityKind(r.Name) == protocol.DirectoryUser {
		return false
	}
	return b.identityKind(r.Owner) == protocol.DirectoryCredential
}

// orphan takes one record and everything that hung on it. None of
// UnregisterAnd's guards apply and that is the whole difference: it refuses
// while a name still owns records, and refuses while a queue is not empty,
// because there is somebody to tell and the choice is theirs. Here there is
// nobody — the messages are addressed to something no principal answers for,
// and holding them only means holding them forever.
// Caller holds b.mu.
func (b *Bus) orphan(name string) {
	// Released before the inbox goes, and with nothing out of it: a reader
	// waiting on a name that is being deleted is owed the answer that will be
	// true a moment later, not a message from a queue that is being thrown
	// away. Nothing else releases them, for two reasons in sequence — after
	// the delete recheckReaders cannot enumerate this inbox at all, and even
	// before it a deleted record reads back as the zero record, whose empty
	// allow list makes may() true for everybody. The waits the same name left
	// on inboxes that SURVIVE are a different problem, collected by the
	// recheckReaders at the end of Orphans.
	if in := b.inboxes[name]; in != nil {
		for _, w := range in.waiters {
			w.stopped <- fmt.Errorf("%w: %s", ErrUnknown, name)
		}
	}
	b.forgetName(name)
}
