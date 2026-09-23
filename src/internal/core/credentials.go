package core

import (
	"fmt"
	"sort"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A credential names a User, and an agent's also names the Agent whose Owner
// that User is, by internal ID (ports.CredentialPair). Every call checks the
// pair against the registry as it is now: a transfer rebinds it in the
// transfer's own commit, a removal deletes it in the removal's, and a pair
// that disagrees can only be left by a failed write — ignored, never repaired.
// See docs/02-access.md#what-a-call-carries.

// pairFor is what name's credential must answer for. Only a User and an Agent
// hold credentials: a queue, a topic and a service are reached, never speak.
// Caller holds b.mu.
func (b *Bus) pairFor(name string) (ports.CredentialPair, error) {
	if u, user := b.users[name]; user {
		return ports.CredentialPair{UserID: u.ID}, nil
	}
	r, known := b.records[name]
	if !known {
		return ports.CredentialPair{}, fmt.Errorf("%w: %s has no profile and no record of its own", ErrNoPrincipal, name)
	}
	if r.Kind != protocol.KindAgent {
		return ports.CredentialPair{}, fmt.Errorf("%w: a %s holds no credential; only a user and an agent speak on the bus", ErrKind, r.Kind)
	}
	owner, user := b.users[r.Owner]
	if !user {
		return ports.CredentialPair{}, fmt.Errorf("%w: %s's owner %s is not a user", ErrNoPrincipal, name, r.Owner)
	}
	return ports.CredentialPair{UserID: owner.ID, AgentID: r.ID}, nil
}

// PairFor is pairFor for a face minting outside a registry hold, where the
// answer cannot go stale: a User's pair never changes.
func (b *Bus) PairFor(name string) (ports.CredentialPair, error) {
	n, err := canon(name)
	if err != nil {
		return ports.CredentialPair{}, err
	}
	b.mu.Lock()
	defer b.unlock()
	return b.pairFor(n)
}

// BindCredentials connects the face's credentials and checks every one held
// against the registry just loaded. A credential for a name that is nothing
// is left to the ownerless sweep; one issued unbound is bound now; one whose
// pair disagrees, or that a kind which holds none carries, is ignored and
// reported as an alert — it authenticates nothing and stays in the store for
// an operator. The report names the User, the Agent and the current Owner,
// never the credential.
func (b *Bus) BindCredentials(idx ports.CredentialIndex) {
	b.mu.Lock()
	defer b.unlock()
	b.creds = idx
	idx.PairOf(func(name string) (ports.CredentialPair, error) { return b.PairFor(name) })
	pairs := idx.Pairs()
	names := make([]string, 0, len(pairs))
	for name := range pairs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		got := pairs[name]
		_, user := b.users[name]
		_, record := b.records[name]
		if !user && !record {
			continue
		}
		want, err := b.pairFor(name)
		switch {
		case err != nil:
			b.report(ports.Alert, "stored credential for %s is ignored: %s", name, err)
			idx.Discard(name)
		case got == (ports.CredentialPair{}):
			if err := idx.Bind(name, want); err != nil {
				b.report(ports.Error, "credential for %s could not be bound: %s", name, err)
			}
		case got != want:
			b.report(ports.Alert, "stored credential for %s is ignored: it names %s, and %s", name, b.describePair(got), b.describeHolder(name))
			idx.Discard(name)
		}
	}
}

// CheckCredential answers whether a credential still answers for name. A
// credential issued unbound is bound on this first check; any other mismatch
// refuses the call as a bad credential, as if it did not exist.
func (b *Bus) CheckCredential(name string, got ports.CredentialPair) error {
	n, err := canon(name)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.unlock()
	want, err := b.pairFor(n)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrStaleCredential, err)
	}
	if got == (ports.CredentialPair{}) && b.creds != nil {
		if err := b.creds.Bind(n, want); err != nil {
			return err
		}
		return nil
	}
	if got != want {
		return fmt.Errorf("%w: it was issued for %s, and %s", ErrStaleCredential, b.describePair(got), b.describeHolder(n))
	}
	return nil
}

// stageCredential puts a credential change into the write in progress: a new
// pair for a transfer, nil for a removal. It commits with the write and
// reaches the index only after it. Caller holds b.mu.
func (b *Bus) stageCredential(name string, p *ports.CredentialPair) {
	s := b.stage()
	if s.creds == nil {
		s.creds = map[string]*ports.CredentialPair{}
	}
	s.creds[name] = p
}

// publishCredentials is the in-memory half of a committed write's
// credentials. Caller holds b.mu.
func (b *Bus) publishCredentials(creds map[string]*ports.CredentialPair) {
	if b.creds == nil {
		return
	}
	for name, p := range creds {
		if p == nil {
			b.creds.Discard(name)
		} else {
			b.creds.Rebind(name, *p)
		}
	}
}

// describePair names the IDs of a pair by who holds them now. Caller holds b.mu.
func (b *Bus) describePair(p ports.CredentialPair) string {
	user := fmt.Sprintf("user %d", p.UserID)
	if name, ok := b.userByID[p.UserID]; ok {
		user += " (" + name + ")"
	}
	if p.AgentID == 0 {
		return user
	}
	agent := fmt.Sprintf("agent %d", p.AgentID)
	if name, ok := b.recordByID[p.AgentID]; ok {
		agent += " (" + name + ")"
	}
	return user + " and " + agent
}

// describeHolder says who name is now. Caller holds b.mu.
func (b *Bus) describeHolder(name string) string {
	if u, user := b.users[name]; user {
		return fmt.Sprintf("%s is user %d", name, u.ID)
	}
	if r, known := b.records[name]; known {
		return fmt.Sprintf("%s is %s %d owned by %s", name, r.Kind, r.ID, r.Owner)
	}
	return name + " is nobody"
}
