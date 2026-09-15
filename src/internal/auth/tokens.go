// Package auth holds the credentials a name is checked against: a token per
// principal, saved so a restart does not strand a queue that was filled under
// the old one. It knows nothing about the bus — a token maps to a name, and
// what that name may do is the registry's business.
// See docs/02-access.md#token-lifetime.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// held is what one principal has: the token to hand out, and the one before
// it. Both authenticate — a refresh must not strand messages already queued
// under the older one — and the one before *that* is gone.
// See docs/02-access.md#token-lifetime.
type held struct {
	current, previous string
	// When it was minted, and when it was last accepted. Issued is durable;
	// used is this run's, like uptime and the envelope feed — writing the
	// store on every authenticated call would put a disk write on the hot
	// path to record something nobody reads more than once a day.
	// See docs/02-access.md#token-lifetime.
	issued time.Time
	used   *atomic.Int64 // unix nanoseconds; zero means not yet, this run
}

// Tokens is the whole credential store, in memory over a store port: what
// it is kept in is an adapter's business, and swapping a file for a database
// does not reach this file (docs/10-modules.md#the-rule).
type Tokens struct {
	mu    sync.RWMutex
	store ports.TokenStore
	who   map[string]string // token -> principal, current and previous alike
	tok   map[string]held   // principal -> what it holds
	// Sessions are credentials too, and deliberately not in the two maps
	// above: those are saved and these are never written down (sessions.go).
	sess map[string]*session
}

// Load reads the store, creating a token for owner on first run.
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
		n, err := protocol.ParseName(c.Name)
		if err != nil {
			return nil, fmt.Errorf("credential store: %w", err)
		}
		t.keep(n.String(), held{current: c.Current, previous: c.Previous, issued: c.Issued})
	}
	if _, has := t.tok[me.String()]; has {
		return t, nil
	}
	if _, err := t.Issue(me.String()); err != nil {
		return nil, err
	}
	return t, nil
}

// Principal says whose credential this is — a token, or a session standing
// for the same person. An unknown one backs nobody, which is not the same as
// backing everybody: the caller must treat false as a refusal.
func (t *Tokens) Principal(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	who, ok := t.who[token]
	if ok {
		if h := t.tok[who]; h.used != nil {
			h.used.Store(time.Now().UnixNano())
		}
		return who, true
	}
	// One lookup for every kind of credential, so there is no route that
	// checks tokens and forgets sessions (docs/05-discovery.md#signing-in).
	return t.session(token)
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
// still authenticates; the one before that is dropped.
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
	fresh := held{
		current: hex.EncodeToString(raw[:]), previous: was.current,
		issued: time.Now(), used: &atomic.Int64{},
	}
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
	if h.used == nil {
		h.used = &atomic.Int64{}
	}
	t.tok[name] = h
	t.who[h.current] = name
	if h.previous != "" {
		t.who[h.previous] = name
	}
}

// Held is what a person may be told about one of their own credentials. The
// token itself is never in it: a page that renders a credential is a page
// that leaks one (docs/05-discovery.md#rules-it-is-built-to).
type Held struct {
	Name        string    `json:"name"`
	Fingerprint string    `json:"fingerprint"`
	Issued      time.Time `json:"issued,omitempty"`
	Used        time.Time `json:"used,omitempty"` // this run's; absent until it is used
	// Whose it is and what it is for, filled in by the caller that knows the
	// registry. A person holds their own name; everything else is a service
	// they registered, and says so (docs/05-discovery.md#dashboard).
	Owner string `json:"owner,omitempty"`
	Kind  string `json:"kind,omitempty"`
}

// Fingerprint names a credential without being one. Keyed, so that a leaked
// fingerprint cannot be checked against a guessed token — an unkeyed digest
// of a 24-byte secret is safe by size alone, and relying on that is the kind
// of reasoning that stops being true when the secret gets shorter.
func fingerprint(token string) string {
	sum := hmac.New(sha256.New, []byte("agent-bus credential fingerprint"))
	sum.Write([]byte(token))
	return hex.EncodeToString(sum.Sum(nil))[:16]
}

// Holds answers, for each name given, what that name's credential looks like
// from outside. Names with no credential are simply absent — the caller asks
// about the names it owns, and owning one does not mean holding one.
// See docs/02-access.md#token-lifetime.
func (t *Tokens) Holds(names []string) []Held {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]Held, 0, len(names))
	for _, n := range names {
		h, has := t.tok[n]
		if !has {
			continue
		}
		held := Held{Name: n, Fingerprint: fingerprint(h.current), Issued: h.issued}
		if h.used != nil {
			if ns := h.used.Load(); ns > 0 {
				held.Used = time.Unix(0, ns)
			}
		}
		out = append(out, held)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Forget drops a principal's credential. It goes with the address: a name
// that is no longer registered answers for nothing, and a credential left
// behind is both clutter in its holder's list and a thing that still
// authenticates (docs/02-access.md#token-lifetime). Absent is success — the
// caller asked for it to be gone. Sessions live in their own map, untouched.
func (t *Tokens) Forget(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	h, has := t.tok[name]
	if !has {
		return nil
	}
	delete(t.who, h.current)
	if h.previous != "" {
		delete(t.who, h.previous)
	}
	delete(t.tok, name)
	return t.save()
}

// save hands the whole set to the store. Writing all of it every time is
// what keeps the port this small — there is no update, only the current
// truth.
func (t *Tokens) save() error {
	creds := make([]ports.Credential, 0, len(t.tok))
	for name, h := range t.tok {
		creds = append(creds, ports.Credential{
			Name: name, Current: h.current, Previous: h.previous, Issued: h.issued,
		})
	}
	return t.store.Save(creds)
}

// Names returns principal names only, for the daemon's filtered people view.
func (t *Tokens) Names() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	names := make([]string, 0, len(t.tok))
	for name := range t.tok {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
