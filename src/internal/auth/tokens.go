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
	// When it was minted, and when it was last accepted. Both are durable;
	// used reaches the store only in FlushUsed's batch — writing the store on
	// every authenticated call would put a disk write on the hot path to
	// record something nobody reads more than once a day.
	// See docs/02-access.md#token-lifetime.
	issued time.Time
	used   *atomic.Int64 // unix nanoseconds; zero means never
	// flushed is the last-use time the store holds, so a flush writes only
	// the credentials that were used since.
	flushed int64
	// pair is who this credential answers for (ports.Credential).
	pair ports.CredentialPair
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
	// pairOf binds a credential issued outside core; nil until core is bound.
	pairOf atomic.Pointer[func(string) (ports.CredentialPair, error)]
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
		h := held{current: c.Current, previous: c.Previous, issued: c.Issued, pair: c.CredentialPair, used: &atomic.Int64{}}
		if !c.Used.IsZero() {
			h.used.Store(c.Used.UnixNano())
			h.flushed = c.Used.UnixNano()
		}
		t.keep(n.String(), h)
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
	who, _, ok := t.Credential(token)
	return who, ok
}

// Credential is Principal with what the credential answers for, which the
// caller checks against the registry before trusting the name. A session
// answers for whatever its token did; it carries no pair of its own, and the
// zero pair it returns is bound like any unbound credential.
func (t *Tokens) Credential(token string) (string, ports.CredentialPair, bool) {
	if token == "" {
		return "", ports.CredentialPair{}, false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	who, ok := t.who[token]
	if ok {
		h := t.tok[who]
		if h.used != nil {
			h.used.Store(time.Now().UnixNano())
		}
		return who, h.pair, true
	}
	// One lookup for every kind of credential, so there is no route that
	// checks tokens and forgets sessions (docs/05-discovery.md#signing-in).
	who, ok = t.session(token)
	if !ok {
		return "", ports.CredentialPair{}, false
	}
	return who, t.tok[who].pair, true
}

// Issue hands out name's token, making one if it has none. Asking again is a
// read, not a rotation: the same credential comes back, so a second call
// cannot lock out a service that is already using the first. The pair comes
// from core when it is bound, and is left for binding otherwise.
// See docs/02-access.md#token-lifetime.
func (t *Tokens) Issue(name string) (string, error) {
	return t.IssuePair(name, t.pairFor(name))
}

// IssuePair is Issue with the pair core decided while it still holds.
func (t *Tokens) IssuePair(name string, pair ports.CredentialPair) (string, error) {
	n, err := protocol.ParseName(name)
	if err != nil {
		return "", err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if h, has := t.tok[n.String()]; has {
		return h.current, nil
	}
	return t.mint(n.String(), held{}, pair)
}

// Rotate issues a fresh token and demotes the current one to previous, which
// still authenticates; the one before that is dropped.
// See docs/02-access.md#token-lifetime.
func (t *Tokens) Rotate(name string) (string, error) {
	return t.RotatePair(name, t.pairFor(name))
}

// RotatePair is Rotate with the pair core decided while it still holds.
func (t *Tokens) RotatePair(name string, pair ports.CredentialPair) (string, error) {
	n, err := protocol.ParseName(name)
	if err != nil {
		return "", err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.mint(n.String(), t.tok[n.String()], pair)
}

// pairFor asks core, outside this index's lock: core calls into the index
// while holding its own, so asking it from inside ours would invert the
// order. Unbound when core is not bound yet or the name is no principal.
func (t *Tokens) pairFor(name string) ports.CredentialPair {
	if f := t.pairOf.Load(); f != nil {
		if p, err := (*f)(name); err == nil {
			return p
		}
	}
	return ports.CredentialPair{}
}

// mint makes a token for name, keeping was.current as the previous one, and
// writes the store before handing anything back. Caller holds the lock.
func (t *Tokens) mint(name string, was held, pair ports.CredentialPair) (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	fresh := held{
		current: hex.EncodeToString(raw[:]), previous: was.current,
		issued: time.Now(), used: &atomic.Int64{}, pair: pair,
	}
	// A credential that was not written down is one a restart forgets, so it
	// must not be handed out either. Written first, so there is nothing to put
	// back: on a failure the maps were never touched.
	if err := t.store.Put(credential(name, fresh)); err != nil {
		return "", err
	}
	t.keep(name, fresh)
	return fresh.current, nil
}

func credential(name string, h held) ports.Credential {
	c := ports.Credential{Name: name, Current: h.current, Previous: h.previous, Issued: h.issued, CredentialPair: h.pair}
	if h.used != nil {
		if ns := h.used.Load(); ns > 0 {
			c.Used = time.Unix(0, ns)
		}
	}
	return c
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
	Used        time.Time `json:"used,omitempty"` // as of the last flush or this run; absent until used
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
// caller asked for it to be gone.
//
// **Written before it takes effect.** Dropping the maps first and then failing
// to write leaves a credential that has stopped working and comes back at the
// next restart: a revocation that un-revokes itself, and nobody is told. On a
// failed write nothing changes at all and the error is the answer.
//
// **Browser sessions for the name go too.** A session is a credential without
// being a token (sessions.go), so one that outlived the credential it came
// from would be exactly "no registration, no access" not holding
// (docs/01-identity-and-roles.md#unregistering) — for up to IdleLife, on a name the
// daemon has already decided answers for nothing. They are dropped even when
// there was no token to forget, because a session that stands for a name is
// reachable however the name lost its credential.
func (t *Tokens) Forget(name string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	h, has := t.tok[name]
	if !has {
		t.endSessionsFor(name)
		return nil
	}
	if err := t.store.Drop(name); err != nil {
		return err
	}
	delete(t.who, h.current)
	if h.previous != "" {
		delete(t.who, h.previous)
	}
	delete(t.tok, name)
	t.endSessionsFor(name)
	return nil
}

// endSessionsFor drops every session standing for one name. A session is keyed
// by its own id and not by who it is for, so this is a scan; sign-ins are rare
// and bounded, which is the same reason StartSession sweeps idle ones inline.
// Caller holds the write lock.
func (t *Tokens) endSessionsFor(name string) {
	for id, s := range t.sess {
		if s.who == name {
			delete(t.sess, id)
		}
	}
}

// Pairs returns what every held credential answers for (ports.CredentialIndex).
func (t *Tokens) Pairs() map[string]ports.CredentialPair {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]ports.CredentialPair, len(t.tok))
	for name, h := range t.tok {
		out[name] = h.pair
	}
	return out
}

// Bind persists and publishes the pair of a credential issued unbound.
func (t *Tokens) Bind(name string, p ports.CredentialPair) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	h, has := t.tok[name]
	if !has {
		return nil
	}
	h.pair = p
	if err := t.store.Put(credential(name, h)); err != nil {
		return err
	}
	t.tok[name] = h
	return nil
}

// Rebind publishes a pair that a committed change has already written.
func (t *Tokens) Rebind(name string, p ports.CredentialPair) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if h, has := t.tok[name]; has {
		h.pair = p
		t.tok[name] = h
	}
}

// Discard stops a credential authenticating, and every session standing for
// its name, without writing: the row went in a committed change, or is being
// ignored at load and stays for an operator.
func (t *Tokens) Discard(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if h, has := t.tok[name]; has {
		delete(t.who, h.current)
		if h.previous != "" {
			delete(t.who, h.previous)
		}
		delete(t.tok, name)
	}
	t.endSessionsFor(name)
}

// PairOf is how core binds a credential issued outside it.
func (t *Tokens) PairOf(f func(string) (ports.CredentialPair, error)) {
	t.pairOf.Store(&f)
}

// FlushUsed writes the last-use times that moved since the last flush, as one
// batch, on the queue flush's cadence: a disk write per authenticated call is
// not a price anybody pays for a timestamp (docs/02-access.md#token-lifetime).
func (t *Tokens) FlushUsed() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	moved := map[string]time.Time{}
	for name, h := range t.tok {
		if ns := h.used.Load(); ns > h.flushed {
			moved[name] = time.Unix(0, ns)
		}
	}
	if err := t.store.Touch(moved); err != nil {
		return err
	}
	for name, at := range moved {
		h := t.tok[name]
		h.flushed = at.UnixNano()
		t.tok[name] = h
	}
	return nil
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
