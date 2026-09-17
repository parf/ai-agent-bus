package core

import "github.com/parf/ai-agent-bus/internal/protocol"

// The two ACL layers, in one place because every verb goes through them and a
// second copy is how one path stays open. Service first, then master.
// See docs/02-access.md#acl.

// Masters sets who holds the master ACL. Not a constructor argument: the bus
// is built before the daemon has parsed its flags, and this is configuration,
// not domain state.
func (b *Bus) Masters(names []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.masters = map[string]bool{}
	for _, n := range names {
		if c, err := canon(n); err == nil {
			b.masters[c] = true
		}
	}
}

// may answers whether caller may see and use r. Caller holds the lock.
//
// A record is always its owner's and its own — a service that could not read
// the inbox it registered would be unable to start. An empty ACL grants nobody
// else access. With an explicit ACL, matching entries and master may grant
// access to other principals.
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
	// Master does not reopen an empty ACL.
	if len(r.Allow) == 0 {
		return false
	}
	for _, s := range r.Allow {
		if s == "*" || s == caller || b.member(caller, s) {
			return true
		}
	}
	return !r.NoMaster && (caller == b.admin || b.masters[caller])
}
