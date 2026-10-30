// Package locks is the daemon's shared locks: named locks on a record, one
// holder at a time, every one with a ttl, memory only (a restart releases
// every lock). The record is the namespace; who may use its locks is resolved
// by the caller, which keeps this package free of the registry
// (docs/01-identity-and-authority.md#shared-locks).
package locks

import (
	"context"
	"sync"
	"time"
)

type key struct{ record, name string }

// A pending take belongs to the record lifetime it started in.
type attempt struct{ removed bool }

// MaxHeld is how many locks one record holds at once: a lock is memory, and
// every caller that may lock a record could otherwise grow it without end
// (issues/done/2026-10/2026-10-17.md#E9).
const MaxHeld = 10000

type held struct {
	holder  string
	expires time.Time
}

// A waiter is one take in a lock's queue. It is handed the lock in its turn,
// granted is set under s.mu, and ch is closed once it is decided: granted, or
// its record gone.
type waiter struct {
	ch      chan struct{}
	holder  string
	ttl     time.Duration
	a       *attempt
	granted bool
}

// sweepEvery is a var so a test can all but stop the sweep and measure
// Take's own expiry check, which a slow holder must not outwait.
var sweepEvery = 100 * time.Millisecond

// Store is the lock table. Zero value is not usable; New starts the expiry
// sweep, and Stop ends it.
type Store struct {
	mu      sync.Mutex
	held    map[key]*held
	per     map[string]int
	waiters map[key][]*waiter
	taking  map[string]map[*attempt]struct{}
	stop    chan struct{}
	done    chan struct{}
}

func New() *Store {
	s := &Store{
		held:    map[key]*held{},
		per:     map[string]int{},
		waiters: map[key][]*waiter{},
		taking:  map[string]map[*attempt]struct{}{},
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

// expire removes a hold and hands the lock to the oldest waiter. Caller
// holds s.mu.
func (s *Store) expire(k key) {
	if _, ok := s.held[k]; ok {
		delete(s.held, k)
		if s.per[k.record]--; s.per[k.record] <= 0 {
			delete(s.per, k.record)
		}
	}
	s.handOff(k)
}

// handOff grants a free lock to the head of its queue, so waiters are served
// in arrival order and a take that arrives at the moment of release cannot
// overtake them. A waiter whose record went is told so and skipped. Caller
// holds s.mu.
func (s *Store) handOff(k key) {
	for len(s.waiters[k]) > 0 {
		w := s.waiters[k][0]
		s.waiters[k] = s.waiters[k][1:]
		if !w.a.removed {
			w.granted = true
			s.held[k] = &held{holder: w.holder, expires: time.Now().Add(w.ttl)}
			s.per[k.record]++
		}
		close(w.ch)
		if w.granted {
			break
		}
	}
	if len(s.waiters[k]) == 0 {
		delete(s.waiters, k)
	}
}

// Take is try-lock and lock in one: with no wait it answers now, with one it
// waits until the lock is granted, the wait runs out or ctx ends. The answer
// names the holder when it is somebody else's, including the caller itself
// (re-taking your own lock is refused at once, never waited on).
func (s *Store) Take(ctx context.Context, record, name, holder string, ttl, wait time.Duration) (bool, string) {
	granted, who, _ := s.Acquire(ctx, record, name, holder, ttl, wait)
	return granted, who
}

// Acquire is Take that also says when the record went while it waited, so a
// name registered again inherits no waiter. Authority is the caller's to
// decide once, before it asks: a wait never re-asks it
// (docs/constitution.md#authority-at-the-door).
func (s *Store) Acquire(ctx context.Context, record, name, holder string, ttl, wait time.Duration) (granted bool, who string, err error) {
	if ttl <= 0 {
		ttl = time.Minute
	}
	k := key{record, name}
	a := &attempt{}
	s.mu.Lock()
	if s.taking[record] == nil {
		s.taking[record] = map[*attempt]struct{}{}
	}
	s.taking[record][a] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.taking[record], a)
		if len(s.taking[record]) == 0 {
			delete(s.taking, record)
		}
		s.mu.Unlock()
	}()
	deadline := time.Now().Add(wait)
	for {
		if err := ctx.Err(); err != nil {
			return false, who, nil
		}
		s.mu.Lock()
		if a.removed {
			s.mu.Unlock()
			return false, "", ErrRecordGone
		}
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
				return false, who, nil
			}
			left := time.Until(deadline)
			if left <= 0 {
				s.mu.Unlock()
				return false, who, nil
			}
			w := &waiter{ch: make(chan struct{}), holder: holder, ttl: ttl, a: a}
			s.waiters[k] = append(s.waiters[k], w)
			s.mu.Unlock()
			cancelled := false
			select {
			case <-w.ch:
			case <-ctx.Done():
				cancelled = true
			case <-time.After(left):
			}
			if s.leave(k, w, cancelled) {
				return true, "", nil
			}
			if !w.granted && isClosed(w.ch) {
				continue // its record went: the loop says so
			}
			return false, who, nil
		}
		if s.per[record] >= MaxHeld {
			s.mu.Unlock()
			return false, "", ErrTooMany
		}
		s.held[k] = &held{holder: holder, expires: time.Now().Add(ttl)}
		s.per[record]++
		s.mu.Unlock()
		return true, "", nil
	}
}

// SelfTake is the refusal a holder gets for taking what they already hold:
// refused at once with the way forward, never waited on.
type SelfTake struct{}

func (*SelfTake) Error() string        { return "you already hold it; use extend" }
func (*SelfTake) Is(target error) bool { _, ok := target.(*SelfTake); return ok }

// leave settles a waiter that stopped waiting, and says whether it holds the
// lock. One that gave up leaves the queue. One handed the lock just as its own
// wait ran out keeps it; one whose caller went away gives it to the next.
func (s *Store) leave(k key, w *waiter, cancelled bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w.granted {
		if !cancelled {
			return true
		}
		if e, ok := s.held[k]; ok && e.holder == w.holder {
			s.expire(k)
		}
		return false
	}
	for i, q := range s.waiters[k] {
		if q == w {
			s.waiters[k] = append(s.waiters[k][:i:i], s.waiters[k][i+1:]...)
			if len(s.waiters[k]) == 0 {
				delete(s.waiters, k)
			}
			break
		}
	}
	return false
}

func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// Release gives a lock back. The holder may; --force is how one pipeline
// stage releases what a later one took, and the answer names who was
// displaced so the audit can say it.
func (s *Store) Release(record, name, holder string, force bool) error {
	k := key{record, name}
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
func (s *Store) Extend(record, name, holder string, ttl time.Duration) error {
	k := key{record, name}
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

// Lock is one hold, for the web face's "time left" column.
type Lock struct {
	Holder  string    `json:"holder"`
	Expires time.Time `json:"expires"`
}

// Holders lists a record's locks, purged of what expired, with each hold's
// expiry so a page can show the time left.
func (s *Store) Holders(record string) map[string]Lock {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	out := map[string]Lock{}
	for k, e := range s.held {
		if k.record != record {
			continue
		}
		if now.After(e.expires) {
			s.expire(k)
			continue
		}
		out[k.name] = Lock{Holder: e.holder, Expires: e.expires}
	}
	return out
}

// DropWhere forgets the holds on every record gone says is gone, and wakes
// their waiters: a deactivation ends that record's locks now, and nobody
// else's (docs/01-identity-and-authority.md#shared-locks).
func (s *Store) DropWhere(gone func(record string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropWhere(gone)
}

// Change serializes a registry change with acquisition and cleanup. A failed
// durable change leaves its locks intact. Callbacks must not re-enter Store.
func (s *Store) Change(change func() error, gone func(string) bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := change(); err != nil {
		return err
	}
	if gone != nil {
		s.dropWhere(gone)
	}
	return nil
}

func (s *Store) dropWhere(gone func(string) bool) {
	records := map[string]bool{}
	for k := range s.held {
		records[k.record] = true
	}
	for record := range s.taking {
		records[record] = true
	}
	for record := range records {
		if gone(record) {
			s.dropRecord(record)
		}
	}
}

// All lists every hold, by record, purged of what expired.
func (s *Store) All() map[string]map[string]Lock {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	out := map[string]map[string]Lock{}
	for k, e := range s.held {
		if now.After(e.expires) {
			s.expire(k)
			continue
		}
		if out[k.record] == nil {
			out[k.record] = map[string]Lock{}
		}
		out[k.record][k.name] = Lock{Holder: e.holder, Expires: e.expires}
	}
	return out
}

// DropRecord deletes all holds on a removed record and refuses its pending
// takes, so a name registered again cannot inherit a hold or an old waiter.
func (s *Store) DropRecord(record string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropRecord(record)
}

func (s *Store) dropRecord(record string) {
	for a := range s.taking[record] {
		a.removed = true
	}
	for k := range s.held {
		if k.record == record {
			s.expire(k)
		}
	}
}
