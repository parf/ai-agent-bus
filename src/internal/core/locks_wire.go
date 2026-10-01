// Thin wiring: the registry answers what the lock table cannot — is the record
// there and active, and does the caller manage it: its Owner, a Maintainer, or
// its own principal (docs/01-identity-and-authority.md#shared-locks).
package core

// LockAccess says whether record is live, and whether caller may use its
// locks. Caller holds nothing.
func (b *Bus) LockAccess(caller, record string) (live, may bool) {
	b.mu.Lock()
	defer b.unlock()
	r, ok := b.entity(record)
	return ok, ok && b.resourceManages(caller, r)
}

// LockKind is LockAccess's answer with the record's kind, for a listing of
// every lock the caller may use. Caller holds nothing.
func (b *Bus) LockKind(caller, record string) (kind string, may bool) {
	b.mu.Lock()
	defer b.unlock()
	r, ok := b.entity(record)
	if !ok || !b.resourceManages(caller, r) {
		return "", false
	}
	return r.Kind, true
}
