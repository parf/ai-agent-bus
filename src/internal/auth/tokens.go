// Package auth holds the credentials a name is checked against: one token per
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

// Tokens is the whole credential store. The file behind it is one line per
// principal — `name token` — because that is all there is to keep and it
// stays readable to the account that owns it.
type Tokens struct {
	mu   sync.RWMutex
	path string
	who  map[string]string // token -> principal
	tok  map[string]string // principal -> token
}

// Load reads the store, creating it with a token for owner on first run. A
// file holding a single bare token is the one the PoC wrote: it is the
// owner's, and saying so is cheaper than asking anyone to migrate by hand.
func Load(path, owner string) (*Tokens, error) {
	me, err := protocol.ParseName(owner)
	if err != nil {
		return nil, fmt.Errorf("owner: %w", err)
	}
	t := &Tokens{path: path, who: map[string]string{}, tok: map[string]string{}}
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
			t.set(me.String(), f[0])
		default:
			n, err := protocol.ParseName(f[0])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			t.set(n.String(), f[1])
		}
	}
	if _, held := t.tok[me.String()]; held {
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
	if tok, held := t.tok[n.String()]; held {
		return tok, nil
	}
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(raw[:])
	t.set(n.String(), tok)
	if err := t.save(); err != nil {
		// A credential that was not written down is one a restart forgets,
		// so it must not be handed out either. Undoing it is exactly these
		// two deletes: we only get here when the name held nothing, so set
		// displaced no earlier token.
		delete(t.tok, n.String())
		delete(t.who, tok)
		return "", err
	}
	return tok, nil
}

// set records a pair. Caller holds the lock, or is Load before anyone else
// can see it.
func (t *Tokens) set(name, token string) {
	if old, held := t.tok[name]; held {
		delete(t.who, old)
	}
	t.tok[name] = token
	t.who[token] = name
}

// save rewrites the whole file through a temporary one: a half-written store
// is a set of principals who can no longer authenticate.
func (t *Tokens) save() error {
	var b strings.Builder
	for name, tok := range t.tok {
		fmt.Fprintf(&b, "%s %s\n", name, tok)
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
