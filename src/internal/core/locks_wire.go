// Thin wiring: the registry answers what the lock table cannot — is the
// caller a member of the group, and is the group there and active. Two
// exported one-liners over existing internals, nothing else
// (Plans/R1.0-Release/locks.md → docs/01-identity-and-roles.md#shared-locks).
package core

// InGroup says whether caller is an effective member of group, nested groups
// included. Caller holds nothing; the caller does.
func (b *Bus) InGroup(caller, group string) bool {
	b.mu.Lock()
	defer b.unlock()
	return b.member(caller, group)
}

// GroupLive says whether group exists and is active: an inactive group has no
// locks. Caller holds nothing.
func (b *Bus) GroupLive(group string) bool {
	b.mu.Lock()
	defer b.unlock()
	_, ok := b.groupMembers(group)
	return ok
}
