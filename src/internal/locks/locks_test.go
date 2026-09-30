package locks

import (
	"testing"
	"time"
)

func quick(t *testing.T) *Store {
	t.Helper()
	return quickAt(t, 10*time.Second) // the sweep off: Take's own checks are what runs
}

func quickAt(t *testing.T, every time.Duration) *Store {
	t.Helper()
	old := sweepEvery
	sweepEvery = every
	t.Cleanup(func() { sweepEvery = old })
	s := New()
	t.Cleanup(s.Stop)
	return s
}

func fast(t *testing.T) {
	t.Helper()
	old := 100 * time.Millisecond
	// The sweep tick is a constant; the test tolerates it by waiting in
	// multiples of it.
	_ = old
}

// One holder at a time: the second take is refused and names the holder.
func TestOnlyOneHolder(t *testing.T) {
	s := quick(t)
	if ok, who := s.Take("g", "deploy", "alice@h", time.Minute, 0); !ok || who != "" {
		t.Fatalf("first take: %v %q", ok, who)
	}
	ok, who := s.Take("g", "deploy", "bob@h", time.Minute, 0)
	if ok || who != "alice@h" {
		t.Fatalf("second take: %v %q, want refused naming alice", ok, who)
	}
}

// A ttl ends a hold: a crashed holder cannot wedge the rest.
func TestTTLExpiresTheHold(t *testing.T) {
	s := quick(t)
	s.Take("g", "job", "alice@h", 150*time.Millisecond, 0)
	if ok, _ := s.Take("g", "job", "bob@h", time.Minute, 0); ok {
		t.Fatal("the hold was still there before its ttl")
	}
	// The sweep fires within a few ticks of the expiry.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if ok, _ := s.Take("g", "job", "bob@h", time.Minute, 0); ok {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the hold outlived its ttl by more than a second")
}

// Only the holder releases; --force releases anybody's, and both answer.
func TestReleaseRules(t *testing.T) {
	s := quick(t)
	s.Take("g", "db", "alice@h", time.Minute, 0)
	if err := s.Release("g", "db", "bob@h", false); err == nil {
		t.Fatal("a non-holder released somebody else's lock")
	} else if holder, ok := Reason(err); !ok || holder != "alice@h" {
		t.Fatalf("the refusal did not name the holder: %v", err)
	}
	if err := s.Release("g", "db", "alice@h", false); err != nil {
		t.Fatalf("the holder could not release: %v", err)
	}
	s.Take("g", "db", "alice@h", time.Minute, 0)
	if err := s.Release("g", "db", "bob@h", true); err != nil {
		t.Fatalf("force did not release another holder's lock: %v", err)
	}
	if err := s.Release("g", "db", "bob@h", true); err != ErrNotHeld {
		t.Fatalf("releasing what nobody holds: %v", err)
	}
}

// Holders lists what the group holds, and only that group's.
func TestHoldersListsTheGroup(t *testing.T) {
	s := quick(t)
	s.Take("ops", "a", "alice@h", time.Minute, 0)
	s.Take("other", "a", "bob@h", time.Minute, 0)
	got := s.Holders("ops")
	if len(got) != 1 || got["a"] != "alice@h" {
		t.Fatalf("holders(ops) = %v", got)
	}
}

// An inactive group has no locks: what it held is gone, waiters are woken.
func TestDropGroupReleasesEverything(t *testing.T) {
	s := quick(t)
	s.Take("ops", "a", "alice@h", time.Minute, 0)
	s.DropGroup("ops")
	if got := s.Holders("ops"); len(got) != 0 {
		t.Fatalf("a dropped group still holds: %v", got)
	}
	if err := s.Release("g", "a", "alice@h", false); err != ErrNotHeld {
		t.Fatal("unrelated groups were dropped too")
	}
}

// A waiting take is granted when the holder releases, and names the holder
// while it waits.
func TestWaitEndsOnRelease(t *testing.T) {
	s := quick(t)
	s.Take("g", "x", "alice@h", 10*time.Second, 0)
	granted := make(chan bool, 1)
	go func() {
		ok, _ := s.Take("g", "x", "bob@h", time.Minute, 5*time.Second)
		granted <- ok
	}()
	time.Sleep(150 * time.Millisecond)
	if err := s.Release("g", "x", "alice@h", false); err != nil {
		t.Fatal(err)
	}
	if !<-granted {
		t.Fatal("the waiter was not granted after the release")
	}
}

// A waiting take ends at its own deadline with the holder's name.
func TestWaitTimesOut(t *testing.T) {
	s := quick(t)
	s.Take("g", "x", "alice@h", time.Minute, 0)
	start := time.Now()
	ok, who := s.Take("g", "x", "bob@h", time.Minute, 200*time.Millisecond)
	if ok || who != "alice@h" {
		t.Fatalf("timed-out take: %v %q", ok, who)
	}
	if d := time.Since(start); d < 150*time.Millisecond {
		t.Fatalf("returned in %v, before its own wait", d)
	}
}

// A waiter is woken by expiry too, not only by release.
func TestWaitEndsOnExpiry(t *testing.T) {
	s := quickAt(t, 100*time.Millisecond) // the sweep is what wakes this waiter
	s.Take("g", "x", "alice@h", 250*time.Millisecond, 0)
	granted := make(chan bool, 1)
	go func() {
		ok, _ := s.Take("g", "x", "bob@h", time.Minute, 3*time.Second)
		granted <- ok
	}()
	if !<-granted {
		t.Fatal("the waiter was not granted when the hold expired")
	}
}

// Take itself honours the ttl, with the sweep all but stopped: the sweep is
// the fallback, not the only expiry path.
func TestTakeHonoursExpiryItself(t *testing.T) {
	s := quick(t) // sweepEvery is 10s here
	s.Take("g", "x", "alice@h", 60*time.Millisecond, 0)
	time.Sleep(120 * time.Millisecond)
	if ok, who := s.Take("g", "x", "bob@h", time.Minute, 0); !ok || who != "" {
		t.Fatalf("an expired hold refused a take without the sweep: %v %q", ok, who)
	}
}

// The sweep alone releases an expired hold nothing else touches.
func TestTheSweepAloneExpires(t *testing.T) {
	s := quickAt(t, 100*time.Millisecond)
	s.Take("g", "x", "alice@h", 150*time.Millisecond, 0)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(s.Holders("g")) == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the sweep did not release the expired hold")
}
