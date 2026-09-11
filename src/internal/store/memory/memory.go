// Package memory is the store adapter that keeps nothing: credentials live
// for as long as the process does. It is what a test uses instead of a
// temporary directory, and the second implementation that makes the port
// more than a name — swapping one is meant to reach nothing inward
// (docs/10-modules.md#the-rule).
package memory

import (
	"sync"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// Tokens holds the set, seeded with whatever the caller wants already there.
type Tokens struct {
	mu    sync.Mutex
	creds []ports.Credential
}

func NewTokens(seed ...ports.Credential) *Tokens { return &Tokens{creds: seed} }

func (t *Tokens) Load() ([]ports.Credential, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]ports.Credential(nil), t.creds...), nil
}

func (t *Tokens) Save(creds []ports.Credential) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.creds = append([]ports.Credential(nil), creds...)
	return nil
}
