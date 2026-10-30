package locks

import (
	"context"
	"testing"
	"time"
)

// waitQueued waits until n takes are queued on record/name, so a test knows
// the arrival order it set up instead of assuming a sleep made it.
func waitQueued(t *testing.T, s *Store, record, name string, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		got := len(s.waiters[key{record, name}])
		s.mu.Unlock()
		if got == n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%d takes never queued", n)
}

func holderOf(s *Store, record, name string) string {
	return s.Holders(record)[name].Holder
}

// Waiters are granted in the order they arrived, one at a time, each only
// after the one before it released (J27).
func TestWaitersAreGrantedInArrivalOrder(t *testing.T) {
	s := quick(t)
	s.Take(context.Background(), "g", "x", "alice@h", time.Minute, 0)
	granted := make(chan string, 5)
	names := []string{"w1@h", "w2@h", "w3@h", "w4@h", "w5@h"}
	for i, who := range names {
		go func() {
			if ok, _ := s.Take(context.Background(), "g", "x", who, time.Minute, 5*time.Second); ok {
				granted <- who
			} else {
				granted <- "refused " + who
			}
		}()
		waitQueued(t, s, "g", "x", i+1)
	}
	prev := "alice@h"
	for _, want := range names {
		if err := s.Release("g", "x", prev, false); err != nil {
			t.Fatalf("release by %s: %v", prev, err)
		}
		select {
		case got := <-granted:
			if got != want {
				t.Fatalf("granted %q, want %q next in arrival order", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s was not granted after %s released", want, prev)
		}
		select {
		case extra := <-granted:
			t.Fatalf("%q was also granted while %s holds it", extra, want)
		case <-time.After(20 * time.Millisecond):
		}
		prev = want
	}
}

// A release hands the lock to the queue's head at once: a take that arrives
// right after the release finds it held by the waiter, not free.
func TestATakeAtReleaseDoesNotOvertakeAWaiter(t *testing.T) {
	s := quick(t)
	s.Take(context.Background(), "g", "x", "alice@h", time.Minute, 0)
	done := make(chan bool, 1)
	go func() {
		ok, _ := s.Take(context.Background(), "g", "x", "bob@h", time.Minute, 5*time.Second)
		done <- ok
	}()
	waitQueued(t, s, "g", "x", 1)
	if err := s.Release("g", "x", "alice@h", false); err != nil {
		t.Fatal(err)
	}
	if ok, who := s.Take(context.Background(), "g", "x", "carol@h", time.Minute, 0); ok || who != "bob@h" {
		t.Fatalf("a take at the release: granted %v, holder %q; want refused naming bob@h", ok, who)
	}
	if !<-done {
		t.Fatal("the waiter was not granted")
	}
}

// Expiry hands off in order too: the sweep's path is the release's path.
func TestExpiryHandsTheLockToTheOldestWaiter(t *testing.T) {
	s := quickAt(t, 20*time.Millisecond)
	s.Take(context.Background(), "g", "x", "alice@h", 100*time.Millisecond, 0)
	granted := make(chan string, 2)
	for i, who := range []string{"first@h", "second@h"} {
		go func() {
			if ok, _ := s.Take(context.Background(), "g", "x", who, time.Minute, 5*time.Second); ok {
				granted <- who
			}
		}()
		waitQueued(t, s, "g", "x", i+1)
	}
	select {
	case got := <-granted:
		if got != "first@h" {
			t.Fatalf("expiry granted %q, want first@h", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nobody was granted when the hold expired")
	}
	if h := holderOf(s, "g", "x"); h != "first@h" {
		t.Fatalf("holder %q after the expiry, want first@h", h)
	}
}

// A waiter whose wait runs out leaves the queue: the lock goes past it to the
// next waiter, never to a take nobody is waiting on.
func TestAWaiterThatGivesUpLeavesTheQueue(t *testing.T) {
	s := quick(t)
	s.Take(context.Background(), "g", "x", "alice@h", time.Minute, 0)
	quitter := make(chan bool, 1)
	go func() {
		ok, _ := s.Take(context.Background(), "g", "x", "quitter@h", time.Minute, 100*time.Millisecond)
		quitter <- ok
	}()
	waitQueued(t, s, "g", "x", 1)
	ctx, cancel := context.WithCancel(context.Background())
	gone := make(chan bool, 1)
	go func() {
		ok, _ := s.Take(ctx, "g", "x", "gone@h", time.Minute, 5*time.Second)
		gone <- ok
	}()
	waitQueued(t, s, "g", "x", 2)
	patient := make(chan bool, 1)
	go func() {
		ok, _ := s.Take(context.Background(), "g", "x", "patient@h", time.Minute, 5*time.Second)
		patient <- ok
	}()
	waitQueued(t, s, "g", "x", 3)
	if <-quitter {
		t.Fatal("the quitter was granted while alice held it")
	}
	cancel()
	if <-gone {
		t.Fatal("the cancelled take was granted while alice held it")
	}
	waitQueued(t, s, "g", "x", 1)
	if err := s.Release("g", "x", "alice@h", false); err != nil {
		t.Fatal(err)
	}
	select {
	case ok := <-patient:
		if !ok {
			t.Fatal("the patient waiter was refused")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("the patient waiter was not granted; holder %q", holderOf(s, "g", "x"))
	}
	if h := holderOf(s, "g", "x"); h != "patient@h" {
		t.Fatalf("holder %q, want patient@h", h)
	}
}

// A waiter handed the lock just as its caller went away gives it to the next
// waiter rather than holding it, unseen, until the ttl, whichever its select
// saw first.
func TestACancelledWaiterHandedTheLockPassesItOn(t *testing.T) {
	s := quick(t)
	k := key{"g", "x"}
	s.Take(context.Background(), "g", "x", "alice@h", time.Minute, 0)
	a := &attempt{}
	late := &waiter{ch: make(chan struct{}), holder: "late@h", ttl: time.Minute, a: a}
	next := &waiter{ch: make(chan struct{}), holder: "next@h", ttl: time.Minute, a: a}
	s.mu.Lock()
	s.waiters[k] = []*waiter{late, next}
	s.expire(k) // alice's hold ends: late is handed the lock
	s.mu.Unlock()
	if !late.granted {
		t.Fatal("the head was not handed the lock")
	}
	// Its select may have seen the hand-off first: its caller is gone all
	// the same, so it must not keep the lock (review of J27).
	gone, cancel := context.WithCancel(context.Background())
	cancel()
	if s.leave(k, late, gone) {
		t.Fatal("a waiter whose caller went kept the lock handed to it")
	}
	if h := holderOf(s, "g", "x"); h != "next@h" || !next.granted {
		t.Fatalf("holder %q, want next@h after the cancelled one passed it on", h)
	}
	// One whose own wait ran out just as it was handed the lock keeps it.
	s.mu.Lock()
	s.expire(k)
	s.waiters[k] = nil
	s.mu.Unlock()
	timely := &waiter{ch: make(chan struct{}), holder: "timely@h", ttl: time.Minute, a: a}
	s.mu.Lock()
	s.waiters[k] = []*waiter{timely}
	s.handOff(k)
	s.mu.Unlock()
	if !s.leave(k, timely, context.Background()) || holderOf(s, "g", "x") != "timely@h" {
		t.Fatalf("a waiter handed the lock as its wait ran out lost it: %q", holderOf(s, "g", "x"))
	}
}

