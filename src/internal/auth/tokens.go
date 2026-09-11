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
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// held is what one principal has: the token to hand out, and the one before
// it. Both authenticate — a refresh must not strand messages already queued
// under the older one — and the one before *that* is gone.
// See docs/02-access.md#token-lifetime.
type held struct {
	current, previous string
}

// Tokens is the whole credential store. The file behind it is one line per
// principal — `name current [previous]` — because that is all there is to
// keep and it stays readable to the account that owns it.
type Tokens struct {
	mu   sync.RWMutex
	path string
	who  map[string]string // token -> principal, current and previous alike
	tok  map[string]held   // principal -> what it holds
}

// Load reads the store, creating a token for owner on first run. A file
// holding a single bare token is the one the PoC wrote: it is the owner's,
// and saying so is cheaper than asking anyone to migrate by hand.
func Load(path, owner string) (*Tokens, error) {
	me, err := protocol.ParseName(owner)
	if err != nil {
		return nil, fmt.Errorf("owner: %w", err)
	}
	t := &Tokens{path: path, who: map[string]string{}, tok: map[string]held{}}
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		switch len(f) {
		case 0:
			continue
		case 1:
			t.keep(me.String(), held{current: f[0]})
		default:
			n, err := protocol.ParseName(f[0])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			h := held{current: f[1]}
			if len(f) > 2 {
				h.previous = f[2]
			}
			t.keep(n.String(), h)
		}
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

// save rewrites the whole file through a temporary one: a half-written store
// is a set of principals who can no longer authenticate.
func (t *Tokens) save() error {
	var b strings.Builder
	for name, h := range t.tok {
		fmt.Fprintf(&b, "%s %s", name, h.current)
		if h.previous != "" {
			fmt.Fprintf(&b, " %s", h.previous)
		}
		b.WriteByte('\n')
	}
	if err := os.MkdirAll(filepath.Dir(t.path), 0o700); err != nil {
		return err
	}
	tmp := t.path + ".new"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, t.path)
}
