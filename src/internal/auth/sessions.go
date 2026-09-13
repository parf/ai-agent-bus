// Sessions are the dashboard's half of a credential, and they live here
// rather than in the child that uses them: a session map inside the web
// process would be every signed-in person's live credential in the one
// process that is restarted with backoff, so a restart would log everybody
// out and a compromise would take the lot.
// See docs/05-discovery.md#signing-in.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"
	"time"
)

// IdleLife is how long a session survives without being used. It is an idle
// timeout and not a lifetime: a person working is not asked to sign in again,
// and a browser left open overnight is.
const IdleLife = 30 * time.Minute

// session is who it stands for and when it was last accepted. `seen` is
// atomic for the same reason a token's `used` is — resolving a credential
// takes a read lock, and refreshing must not need a write one.
type session struct {
	who  string
	seen *atomic.Int64 // unix nanoseconds
}

// StartSession hands back a credential standing for who, expiring when it
// stops being used. It authenticates exactly as a token does and is not one:
// it is **never written to the store**, so a daemon restart ends every
// session, and dropping it touches nothing the person actually holds.
func (t *Tokens) StartSession(who string) (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(raw[:])
	now := &atomic.Int64{}
	now.Store(time.Now().UnixNano())
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.sess == nil {
		t.sess = map[string]*session{}
	}
	// Swept here rather than on a timer: sign-ins are rare and bounded, and
	// a goroutine that outlives its reason is a thing to remember.
	for old, s := range t.sess {
		if idle(s) {
			delete(t.sess, old)
		}
	}
	t.sess[id] = &session{who: who, seen: now}
	return id, nil
}

// EndSession drops one. Signing out is the holder's to do and nothing checks
// whose it was — holding the credential is the whole of the claim.
func (t *Tokens) EndSession(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.sess, id)
}

// session resolves a credential that is not a token. An idle one is refused
// and left to the next sweep: answering false is what matters, and deleting
// it here would need a write lock on the read path.
func (t *Tokens) session(cred string) (string, bool) {
	s, live := t.sess[cred]
	if !live || idle(s) {
		return "", false
	}
	s.seen.Store(time.Now().UnixNano())
	return s.who, true
}

func idle(s *session) bool {
	return time.Since(time.Unix(0, s.seen.Load())) > IdleLife
}
