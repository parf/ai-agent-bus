// Package auth holds the credentials a name is checked against: a token per
// principal, saved so a restart does not strand a queue that was filled under
// the old one. It knows nothing about the bus — a token maps to a name, and
// what that name may do is the registry's business.
// See docs/02-access.md#token-lifetime.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// held is what one principal has: the token to hand out, and the one before
// it. Both authenticate — a refresh must not strand messages already queued
// under the older one — and the one before *that* is gone.
// See docs/02-access.md#token-lifetime.
type held struct {
	current, previous string
}

// Tokens is the whole credential store, in memory over a store port: what
// it is kept in is an adapter's business, and swapping a file for a database
// does not reach this file (docs/10-modules.md#the-rule).
type Tokens struct {
	mu    sync.RWMutex
	store ports.TokenStore
	who   map[string]string // token -> principal, current and previous alike
	tok   map[string]held   // principal -> what it holds
}

// Load reads the store, creating a token for owner on first run. A nameless
// credential is the bare token the PoC wrote: it is the owner's, and saying
// so is cheaper than asking anyone to migrate by hand.
func Load(store ports.TokenStore, owner string) (*Tokens, error) {
	me, err := protocol.ParseName(owner)
	if err != nil {
		return nil, fmt.Errorf("owner: %w", err)
	}
	t := &Tokens{store: store, who: map[string]string{}, tok: map[string]held{}}
	creds, err := store.Load()
	if err != nil {
		return nil, err
	}
	for _, c := range creds {
		name := me.String()
		if c.Name != "" {
			n, err := protocol.ParseName(c.Name)
			if err != nil {
				return nil, fmt.Errorf("credential store: %w", err)
			}
			name = n.String()
		}
		t.keep(name, held{current: c.Current, previous: c.Previous})
	}
	if _, has := t.tok[me.String()]; has {
		return t, nil
	}
	if _, err := t.Issue(me.String()); err != nil {
		return nil, err
	}
	return t, nil
}

// Principal says whose token this is. An unknown token backs nobody, which is
// not the same as backing everybody — the caller must treat false as a
// refusal.
func (t *Tokens) Principal(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	who, ok := t.who[token]
	return who, ok
}

// Issue hands out name's token, making one if it has none. Asking again is a
// read, not a rotation: the same credential comes back, so a second call
// cannot lock out a service that is already using the first.
// See docs/02-access.md#token-lifetime.
func (t *Tokens) Issue(name string) (string, error) {
	n, err := protocol.ParseName(name)
	if err != nil {
		return "", err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if h, has := t.tok[n.String()]; has {
		return h.current, nil
	}
	return t.mint(n.String(), held{})
}

// Rotate issues a fresh token and demotes the current one to previous, which
// still authenticates. Whatever was previous before is dropped: two are
// accepted, never three.
// See docs/02-access.md#token-lifetime.
func (t *Tokens) Rotate(name string) (string, error) {
	n, err := protocol.ParseName(name)
	if err != nil {
		return "", err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.mint(n.String(), t.tok[n.String()])
}

// mint makes a token for name, keeping was.current as the previous one, and
// writes the store before handing anything back. Caller holds the lock.
func (t *Tokens) mint(name string, was held) (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	fresh := held{current: hex.EncodeToString(raw[:]), previous: was.current}
	t.keep(name, fresh)
	if err := t.save(); err != nil {
		// A credential that was not written down is one a restart forgets,
		// so it must not be handed out either. Putting back what was there
		// is enough: keep dropped exactly those tokens.
		t.keep(name, was)
		delete(t.who, fresh.current)
		return "", err
	}
	return fresh.current, nil
}

// keep records what a principal holds, forgetting whatever it held before.
// Caller holds the lock, or is Load before anyone else can see it.
func (t *Tokens) keep(name string, h held) {
	if old, had := t.tok[name]; had {
		delete(t.who, old.current)
		delete(t.who, old.previous)
	}
	if h.current == "" {
		delete(t.tok, name)
		return
	}
	t.tok[name] = h
	t.who[h.current] = name
	if h.previous != "" {
		t.who[h.previous] = name
	}
}

// save hands the whole set to the store. Writing all of it every time is
// what keeps the port this small — there is no update, only the current
// truth.
func (t *Tokens) save() error {
	creds := make([]ports.Credential, 0, len(t.tok))
	for name, h := range t.tok {
		creds = append(creds, ports.Credential{Name: name, Current: h.current, Previous: h.previous})
	}
	return t.store.Save(creds)
}
