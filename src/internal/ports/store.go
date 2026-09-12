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
// See docs/02-access.md#token-lifetime.
type Credential struct {
	Name     string
	Current  string
	Previous string
	// When this credential was minted. Durable like the token itself: a
	// credential nobody can date is one nobody can decide to retire.
	Issued time.Time
}

// TokenStore is the slice of the store port the design already requires
// durable: credentials outlive a restart or a queue filled under the old one
// is stranded. What *else* the store holds is open, so nothing else is
// declared here.
// See docs/09-setup.md#storage.
type TokenStore interface {
	// Load returns every credential kept. A store that has never been
	// written is empty, not an error.
	Load() ([]Credential, error)
	// Save replaces the lot. Partial writes are the adapter's problem: a
	// half-written store is a set of principals who can no longer
	// authenticate.
	Save([]Credential) error
}
