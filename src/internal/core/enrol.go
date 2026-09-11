package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Enrolment, in two steps, because fetching a public key is not
// authentication: the bus says what it wants signed, and the newcomer proves
// it holds the private half of a key the directory publishes for that login.
// See docs/01-identity.md#registration.

// How long a challenge is worth answering. Short: it is one round trip on the
// same host, and an unanswered one is a name somebody is trying to take.
const challengeLife = 2 * time.Minute

type challenge struct {
	name  string
	login string
	keys  []string
	at    time.Time
}

// Directories says which realms are backed by a directory, and what verifies
// a signature. A realm that has one can only be entered by enrolling — that
// is the whole point, and it is what stops a name being claimed by whoever
// asks first (docs/01-identity.md#ownership).
func (b *Bus) Directories(dirs map[string]ports.Directory, sigs ports.Signatures) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dirs, b.sigs = dirs, sigs
	b.pending = map[string]challenge{}
}

// Challenge starts an enrolment: it looks the login up, keeps what it found,
// and hands back the nonce to sign. The keys are held with the challenge so
// that the answer is checked against what was published when the question was
// asked, not against whatever is published when it comes back.
func (b *Bus) Challenge(name string) (string, error) {
	n, err := protocol.ParseName(name)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrBadName, err)
	}
	b.mu.Lock()
	dir, backed := b.dirs[n.Realm]
	b.mu.Unlock()
	if !backed {
		return "", fmt.Errorf("%w: nothing backs the realm %q, so there is nothing to prove", ErrEnrol, n.Realm)
	}
	keys, err := dir.Keys(n.Local)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrEnrol, err)
	}
	if len(keys) == 0 {
		return "", fmt.Errorf("%w: %s publishes no keys", ErrEnrol, n.Local)
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(raw[:])
	b.mu.Lock()
	defer b.mu.Unlock()
	b.forget(time.Now())
	b.pending[nonce] = challenge{name: n.String(), login: n.Local, keys: keys, at: time.Now()}
	return nonce, nil
}

// Enrol finishes it. The record it writes is **owned by itself**: nobody else
// may re-register over it and nobody else may be handed its credential, which
// is what enrolment is for.
func (b *Bus) Enrol(nonce, signature string) (protocol.Record, error) {
	b.mu.Lock()
	c, waiting := b.pending[nonce]
	sigs := b.sigs
	b.mu.Unlock()
	if !waiting || time.Since(c.at) > challengeLife {
		return protocol.Record{}, fmt.Errorf("%w: no challenge is waiting for that answer", ErrEnrol)
	}
	if sigs == nil {
		return protocol.Record{}, fmt.Errorf("%w: nothing here verifies a signature", ErrEnrol)
	}
	if err := sigs.Verify(c.name, nonce, signature, c.keys); err != nil {
		return protocol.Record{}, fmt.Errorf("%w: %s", ErrEnrol, err)
	}
	// Spent whether or not what follows works: a nonce that can be answered
	// twice is a nonce.
	b.mu.Lock()
	delete(b.pending, nonce)
	b.mu.Unlock()
	return b.register(protocol.Record{Name: c.name, Kind: "agent", Owner: c.name}, true)
}

// forget drops challenges nobody answered. Caller holds the lock.
func (b *Bus) forget(now time.Time) {
	for k, c := range b.pending {
		if now.Sub(c.at) > challengeLife {
			delete(b.pending, k)
		}
	}
}
