// Package ports declares what core needs from the outside world and nothing
// else: an interface here, an implementation in an adapter, and core free to
// be built and tested without either. Nothing in this package may import an
// adapter, a face, or the standard library's outside — a port that touches a
// file has stopped being one.
// See docs/10-modules.md#the-rule.
package ports

import "time"

// Credential is one principal's durable pair: the token to hand out, and the
// one before it, which still authenticates. The store keeps them; what they
// mean is core's business.
//
// Every credential names a User, and an agent's also names the Agent whose
// Owner that User is — by internal ID, so a removed name registered again by
// somebody else can never be answered for by the old bytes. A zero pair was
// issued before its principal was bound and is bound at the next check.
// See docs/02-access.md#token-lifetime.
type Credential struct {
	Name     string
	Current  string
	Previous string
	// When this credential was minted. Durable like the token itself: a
	// credential nobody can date is one nobody can decide to retire.
	Issued time.Time
	// When it last authenticated, as of the last flush; zero means never.
	Used time.Time
	CredentialPair
}

// CredentialPair is who a credential answers for: always a User, and for an
// Agent's credential that Agent too.
type CredentialPair struct {
	UserID, AgentID uint32
}

// TokenStore keeps credentials one row at a time, so writing one principal's
// credential never rewrites, or resurrects, another's.
// See docs/09-setup.md#storage.
type TokenStore interface {
	// Load returns every credential kept. A store that has never been
	// written is empty, not an error.
	Load() ([]Credential, error)
	// Put writes one credential whole, pair included.
	Put(Credential) error
	// Drop removes one credential; absent is success.
	Drop(name string) error
	// Touch records when each named credential last authenticated, in one
	// batch on the queue flush's cadence rather than once per call.
	Touch(map[string]time.Time) error
}

// CredentialIndex is core's view of the credentials a face holds: what each
// answers for, and the in-memory half of a change whose durable half is in a
// Change. Its methods take only the index's own lock, never core's.
type CredentialIndex interface {
	// Pairs returns what every held credential answers for.
	Pairs() map[string]CredentialPair
	// Bind persists and publishes a pair for a credential issued unbound.
	Bind(name string, p CredentialPair) error
	// Rebind publishes a pair a committed Change has already written.
	Rebind(name string, p CredentialPair)
	// Discard stops a credential authenticating, with the sessions standing
	// for it, without writing: a committed Change removed the row, or it is
	// being ignored at load.
	Discard(name string)
	// PairOf tells the index how to bind a credential issued outside core.
	PairOf(func(name string) (CredentialPair, error))
}
