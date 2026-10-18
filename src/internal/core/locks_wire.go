// Thin wiring: the registry answers what the lock table cannot — is the record
// there and active, and does the caller manage it: its Owner, a Maintainer, or
// its own principal (docs/01-identity-and-authority.md#shared-locks).
package core

// LockRecord is a record's name as the lock table keys it and every lock
// answer spells it: trimmed and lower-case, as the key-value store and every
// other name is (issues/done/2026-10/2026-10-17.md#A20). A name that does not
// parse is left as sent, to be no such record.
func LockRecord(record string) string {
	if canonical, err := canon(record); err == nil {
		return canonical
	}
	return record
}

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
