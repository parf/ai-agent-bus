package core

import (
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// index is a credential index that records what core told it.
type index struct {
	mu        sync.Mutex
	held      map[string]ports.CredentialPair
	bound     map[string]ports.CredentialPair
	rebound   map[string]ports.CredentialPair
	discarded []string
	pairOf    func(string) (ports.CredentialPair, error)
	bindErr   error
}

func newIndex() *index {
	return &index{held: map[string]ports.CredentialPair{}, bound: map[string]ports.CredentialPair{}, rebound: map[string]ports.CredentialPair{}}
}

func (x *index) Pairs() map[string]ports.CredentialPair {
	x.mu.Lock()
	defer x.mu.Unlock()
	out := map[string]ports.CredentialPair{}
	for n, p := range x.held {
		out[n] = p
	}
	return out
}
func (x *index) Bind(name string, p ports.CredentialPair) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.bindErr != nil {
		return x.bindErr
	}
	x.bound[name], x.held[name] = p, p
	return nil
}
func (x *index) Rebind(name string, p ports.CredentialPair) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.rebound[name] = p
	if _, has := x.held[name]; has {
		x.held[name] = p
	}
}
func (x *index) Discard(name string) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.discarded = append(x.discarded, name)
	delete(x.held, name)
}
func (x *index) PairOf(f func(string) (ports.CredentialPair, error)) { x.pairOf = f }

func mustPair(t *testing.T, b *Bus, name string) ports.CredentialPair {
	t.Helper()
	p, err := b.PairFor(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
