package core

import (
	"fmt"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// ACL decisions live in one place because every verb goes through them and a
// second copy is how one path stays open.
// See docs/02-access.md#acl.

// may answers whether caller may see and use r. Caller holds the lock.
//
// A record is always its owner's and its own — a service that could not read
// the inbox it registered would be unable to start. An empty ACL grants nobody
// else access. With an explicit ACL, matching entries may grant access to
// other principals.
func (b *Bus) may(caller string, r protocol.Record) bool {
	// Asked here rather than only at the edge: every verb reaches an ACL, so
	// one question here is asked under whatever hold the verb writes under.
	// See acting.
	if b.acting(caller) != nil {
		return false
	}
	if b.resourceManages(caller, r) {
		return true
	}
	// No grants means management access only, including on restored records.
	if len(r.Allow) == 0 {
		return false
	}
	for _, s := range r.Allow {
		if s == "*" || s == caller || b.runtimeTerm(caller, s, r) || b.member(caller, s) {
			return true
		}
	}
	return false
}

// sameResourceOwner resolves @owner in the target record's context. Only an
// agent joins its owner's cohort: a queue, a pub/sub topic and a user are
// destinations or people rather than callers acting for an owner, and a
// service is external and calls nothing here. Ownership is deliberately one
// step, so an agent owned by another agent does not join through a chain.
// See docs/02-access.md#acl.
func (b *Bus) sameResourceOwner(caller, owner string) bool {
	if caller == owner {
		return true
	}
	r, ok := b.records[caller]
	return ok && r.Owner == owner && r.Kind == protocol.KindAgent
}

// mayDeliverToUser is the User delivery rule: a User's inbox takes a message
// from the User itself, and from an Agent whose own ACL admits that User. It
// takes none from another User and none from anything else.
// See docs/constitution.md#-channels. Caller holds b.mu.
func (b *Bus) mayDeliverToUser(from string, user protocol.Record) error {
	if from == user.Name {
		return nil
	}
	sender, known := b.records[from]
	if !known || sender.Kind != protocol.KindAgent {
		return fmt.Errorf("%s may not send to %s: a user takes messages only from agents it may reach: %w", from, user.Name, ErrNotAllow)
	}
	if !b.may(user.Name, sender) {
		return fmt.Errorf("%s may not send to %s: %s's ACL does not admit %s: %w", from, user.Name, from, user.Name, ErrNotAllow)
	}
	return nil
}
