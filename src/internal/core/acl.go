package core

import "github.com/parf/ai-agent-bus/internal/protocol"

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
		if s == "*" || s == caller || (s == OwnerGroup && b.sameResourceOwner(caller, r.Owner)) || b.member(caller, s) {
			return true
		}
	}
	return false
}

// sameResourceOwner resolves @owner in the target record's context. Ownership
// is deliberately one step: a Service owned by another Service does not join
// the human owner's cohort through a chain. Channels are destinations, not
// members of this runtime ACL term.
func (b *Bus) sameResourceOwner(caller, owner string) bool {
	if caller == owner {
		return true
	}
	r, ok := b.records[caller]
	return ok && r.Owner == owner && r.Kind == protocol.KindAgent
}
