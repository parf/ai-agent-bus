// Package locks is the daemon's shared locks: named locks in a Group, one
// holder at a time, every one with a ttl, memory only (a restart releases
// every lock). The Group is the namespace and the ACL; membership itself is
// resolved by the caller, which keeps this package free of the registry
// (docs/01-identity-and-roles.md#shared-locks).
package locks

import (
	"context"
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
// and not at its own. Every expiry goes through expire, the one place a hold
// leaves the table, so no path forgets the waiters.
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
					s.expire(k)
				}
			}
			s.mu.Unlock()
		}
	}
}

// expire removes a hold and wakes its waiters. Caller holds s.mu.
func (s *Store) expire(k key) {
	delete(s.held, k)
	s.wake(k)
}

func (s *Store) wake(k key) {
	for _, ch := range s.waiters[k] {
		close(ch)
	}
	delete(s.waiters, k)
}

// Take is try-lock and lock in one: with no wait it answers now, with one it
// waits until the lock is granted, the wait runs out or ctx ends. The answer
// names the holder when it is somebody else's, including the caller itself
// (re-taking your own lock is refused at once, never waited on).
func (s *Store) Take(ctx context.Context, group, name, holder string, ttl, wait time.Duration) (granted bool, who string) {
	if ttl <= 0 {
		ttl = time.Minute
	}
	k := key{group, name}
	deadline := time.Now().Add(wait)
	for {
		if err := ctx.Err(); err != nil {
			return false, who
		}
		s.mu.Lock()
		if e, ok := s.held[k]; ok && time.Now().After(e.expires) {
			s.expire(k)
		}
		if e, ok := s.held[k]; ok {
			who = e.holder
			// Re-taking your own lock is refused at once with the way
			// forward, never waited on: waiting on yourself is a deadlock
			// till your own ttl.
			if who == holder {
				s.mu.Unlock()
				return false, who
			}
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
			case <-ctx.Done():
				s.dropWaiter(k, ch)
				return false, who
			case <-time.After(left):
				s.dropWaiter(k, ch)
				return false, who
			}
			continue
		}
		s.held[k] = &held{holder: holder, expires: time.Now().Add(ttl)}
		s.mu.Unlock()
		return true, ""
	}
}

// SelfTake is the refusal a holder gets for taking what they already hold:
// refused at once with the way forward, never waited on.
type SelfTake struct{}

func (*SelfTake) Error() string        { return "you already hold it; use extend" }
func (*SelfTake) Is(target error) bool { _, ok := target.(*SelfTake); return ok }

// dropWaiter removes a waiter that gave up. A wake may have closed the
// channel between the deadline and here; then nothing is left to remove.
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
}

// Release gives a lock back. The holder may; --force is how one pipeline
// stage releases what a later one took, and the answer names who was
// displaced so the audit can say it.
func (s *Store) Release(group, name, holder string, force bool) error {
	k := key{group, name}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.held[k]
	if !ok || time.Now().After(e.expires) {
		s.expire(k)
		return ErrNotHeld
	}
	if e.holder == holder {
		s.expire(k)
		return nil
	}
	if !force {
		return &HeldBy{Holder: e.holder}
	}
	s.expire(k)
	return &Displaced{Previous: e.holder}
}

// Extend sets a fresh ttl from now. Only the holder may; a lock nobody holds
// is not extended.
func (s *Store) Extend(group, name, holder string, ttl time.Duration) error {
	k := key{group, name}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.held[k]
	if !ok || time.Now().After(e.expires) {
		s.expire(k)
		return ErrNotHeld
	}
	if e.holder != holder {
		return &HeldBy{Holder: e.holder}
	}
	e.expires = time.Now().Add(ttl)
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
			s.expire(k)
			continue
		}
		out[k.name] = e.holder
	}
	return out
}

// Reset forgets every hold and wakes every waiter: a deactivation ends the
// deactivated thing's locks now, not at the next lazy call
// (docs/01-identity-and-roles.md#shared-locks).
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.held {
		s.expire(k)
	}
}
