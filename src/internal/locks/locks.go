// Package locks is the daemon's shared locks: named locks in a Group, one
// holder at a time, every one with a TTL, memory only (a restart releases
// every lock). The Group is the namespace and the ACL; membership itself is
// resolved by the caller, which keeps this package free of the registry
// (Plans/R1.0-Release/locks.md → docs/01-identity-and-roles.md#shared-locks).
package locks

import (
	"sync"
	"time"
)

type key struct{ group, name string }

type held struct {
	holder  string
	expires time.Time
}

// sweepEvery is a var so a test can all but stop the sweep and measure
// Take's own expiry check, which a slow holder must not outwait.
var sweepEvery = 100 * time.Millisecond

// Store is the lock table. Zero value is not usable; New starts the expiry
// sweep, and Stop ends it.
type Store struct {
	mu      sync.Mutex
	held    map[key]*held
	waiters map[key][]chan struct{}
	stop    chan struct{}
	done    chan struct{}
}

func New() *Store {
	s := &Store{
		held:    map[key]*held{},
		waiters: map[key][]chan struct{}{},
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go s.sweep()
	return s
}

func (s *Store) Stop() { close(s.stop); <-s.done }

// sweep wakes waiters whose lock expired, so a wait ends at the holder's ttl
// and not at its own. It holds nothing: an expired lock is nobody's.
func (s *Store) sweep() {
	defer close(s.done)
	t := time.NewTicker(sweepEvery)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.mu.Lock()
			now := time.Now()
			for k, e := range s.held {
				if now.After(e.expires) {
					delete(s.held, k)
					s.wake(k)
				}
			}
			s.mu.Unlock()
		}
	}
}

func (s *Store) wake(k key) {
	for _, ch := range s.waiters[k] {
		close(ch)
	}
	delete(s.waiters, k)
}

// Take is try-lock and lock in one: with no wait it answers now, with one it
// waits until the lock is granted or the wait runs out. The answer names the
// holder when it is somebody else's.
func (s *Store) Take(group, name, holder string, ttl, wait time.Duration) (granted bool, who string) {
	if ttl <= 0 {
		ttl = time.Minute
	}
	k := key{group, name}
	deadline := time.Now().Add(wait)
	for {
		s.mu.Lock()
		if e, ok := s.held[k]; ok {
			if time.Now().After(e.expires) {
				delete(s.held, k)
			} else {
				who = e.holder
				left := time.Until(deadline)
				if left <= 0 {
					s.mu.Unlock()
					return false, who
				}
				ch := make(chan struct{})
				s.waiters[k] = append(s.waiters[k], ch)
				s.mu.Unlock()
				select {
				case <-ch:
				case <-time.After(left):
					s.dropWaiter(k, ch)
					return false, who
				}
				continue
			}
		}
		s.held[k] = &held{holder: holder, expires: time.Now().Add(ttl)}
		s.mu.Unlock()
		return true, ""
	}
}

// dropWaiter removes a waiter that gave up. The channel may already be closed
// by a wake racing the deadline; a send on a closed channel would panic, so
// the removal happens under the lock and the send is skipped.
func (s *Store) dropWaiter(k key, ch chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, w := range s.waiters[k] {
		if w == ch {
			s.waiters[k] = append(s.waiters[k][:i:i], s.waiters[k][i+1:]...)
			if len(s.waiters[k]) == 0 {
				delete(s.waiters, k)
			}
			return
		}
	}
	// Not found: a wake closed it between the deadline and here. Answered
	// anyway — the lock was checked once more before returning.
}

// Release gives a lock back. Only the holder may, unless force: a member's
// --force is how one pipeline stage releases what a later one took.
func (s *Store) Release(group, name, holder string, force bool) error {
	k := key{group, name}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.held[k]
	if !ok || time.Now().After(e.expires) {
		delete(s.held, k)
		return ErrNotHeld
	}
	if e.holder != holder && !force {
		return &HeldBy{Holder: e.holder}
	}
	delete(s.held, k)
	s.wake(k)
	return nil
}

// Holders lists a group's locks, purged of what expired.
func (s *Store) Holders(group string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	out := map[string]string{}
	for k, e := range s.held {
		if k.group != group {
			continue
		}
		if now.After(e.expires) {
			delete(s.held, k)
			continue
		}
		out[k.name] = e.holder
	}
	return out
}

// DropGroup forgets a group's locks: an inactive group has none
// (docs/constitution.md#common-record-fields).
func (s *Store) DropGroup(group string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.held {
		if k.group == group {
			delete(s.held, k)
			s.wake(k)
		}
	}
}
