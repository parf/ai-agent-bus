package auth

import (
	"fmt"
	"sync/atomic"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// CommitImport holds the credential index over commit: a concurrent rotation
// cannot commit between the imported values and their in-memory publication.
func (t *Tokens) CommitImport(values map[string]ports.Credential, pairs map[string]*ports.CredentialPair, commit func() error) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	seen := map[string]string{}
	for name, c := range values {
		if c.Name != name || c.Current == "" {
			return fmt.Errorf("invalid imported credential for %s", name)
		}
		for _, token := range []string{c.Current, c.Previous} {
			if token == "" {
				continue
			}
			if other, ok := seen[token]; ok && other != name {
				return fmt.Errorf("imported credential conflicts between %s and %s", name, other)
			}
			seen[token] = name
			if other, ok := t.who[token]; ok && other != name {
				return fmt.Errorf("imported credential conflicts with %s", other)
			}
		}
	}
	if err := commit(); err != nil {
		return err
	}
	// Values and pairs have been validated before the only fallible operation.
	for name, p := range pairs {
		if _, full := values[name]; full {
			continue
		}
		if p == nil {
			t.discardLocked(name)
		} else if h, ok := t.tok[name]; ok {
			h.pair = *p
			t.tok[name] = h
		}
	}
	for name, c := range values {
		t.discardLocked(name)
		used := &atomic.Int64{}
		if !c.Used.IsZero() {
			used.Store(c.Used.UnixNano())
		}
		t.keep(name, held{current: c.Current, previous: c.Previous, issued: c.Issued, used: used, flushed: used.Load(), pair: c.CredentialPair})
	}
	return nil
}
func (t *Tokens) discardLocked(name string) {
	if h, ok := t.tok[name]; ok {
		delete(t.who, h.current)
		delete(t.who, h.previous)
		delete(t.tok, name)
	}
	t.endSessionsFor(name)
}
