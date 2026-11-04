package core

import (
	"errors"
	"sync"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// index is a credential index that records what core told it.
type index struct {
	mu        sync.Mutex
	held      map[string]ports.CredentialPair
	rebound   map[string]ports.CredentialPair
	discarded []string
	pairOf    func(string) (ports.CredentialPair, error)
}

func newIndex() *index {
	return &index{held: map[string]ports.CredentialPair{}, rebound: map[string]ports.CredentialPair{}}
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

// No zero pair is converted into authority, for either a User or an Agent.
func TestACorruptCredentialRowIsNeverRepairedSilently(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	rep := &reports{}
	b.Journal(rep)
	known(t, b, "alice@h", "#svc@h")
	idx := newIndex()
	idx.held["#svc@h"] = ports.CredentialPair{}
	idx.held["alice@h"] = ports.CredentialPair{}
	b.BindCredentials(idx)
	if _, held := idx.held["#svc@h"]; held {
		t.Fatal("an agent's empty pair was bound")
	}
	if !rep.has("stored credential for #svc@h is ignored:") {
		t.Fatalf("the empty agent pair was not reported: %v", rep.lines)
	}
	if _, held := idx.held["alice@h"]; held {
		t.Fatal("a zero-pair user credential remained usable")
	}
	if err := b.CheckCredential("alice@h", ports.CredentialPair{}); !errors.Is(err, ErrStaleCredential) {
		t.Fatalf("zero user pair accepted: %v", err)
	}
	// Presented, an agent's empty pair authenticates nothing and binds nothing.
	if err := b.CheckCredential("#svc@h", ports.CredentialPair{}); !errors.Is(err, ErrStaleCredential) {
		t.Fatalf("an agent's empty pair authenticated: %v", err)
	}
	if _, held := idx.held["#svc@h"]; held {
		t.Fatal("checking the empty pair bound it")
	}
	// Issuing again over the ignored row is a repair somebody asked for, and said.
	mint := func(string, ports.CredentialPair) (string, error) { return "t", nil }
	if _, err := b.IssueFor(fixtureOwner, "#svc@h", mint); err != nil {
		t.Fatal(err)
	}
	if !rep.has("stored credential for #svc@h, ignored as corrupt, is replaced by one " + fixtureOwner + " issued") {
		t.Fatalf("the re-issue over a corrupt row was not reported: %v", rep.lines)
	}
}
