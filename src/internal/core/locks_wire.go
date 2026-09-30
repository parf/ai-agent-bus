// Thin wiring: the registry answers what the lock table cannot — is the record
// there and active, and does the caller manage it: its Owner, a Maintainer, or
// its own principal (docs/01-identity-and-roles.md#shared-locks).
package core

// LockAccess says whether record is live, and whether caller may use its
// locks. Caller holds nothing.
func (b *Bus) LockAccess(caller, record string) (live, may bool) {
	b.mu.Lock()
	defer b.unlock()
	r, ok := b.entity(record)
	return ok, ok && b.resourceManages(caller, r)
}
