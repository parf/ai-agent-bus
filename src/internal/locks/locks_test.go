package locks

import (
	"context"
	"errors"
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

// One holder at a time: the second take is refused and names the holder.
func TestOnlyOneHolder(t *testing.T) {
	s := quick(t)
	if ok, who := s.Take(context.Background(), "g", "deploy", "alice@h", time.Minute, 0); !ok || who != "" {
		t.Fatalf("first take: %v %q", ok, who)
	}
	ok, who := s.Take(context.Background(), "g", "deploy", "bob@h", time.Minute, 0)
	if ok || who != "alice@h" {
		t.Fatalf("second take: %v %q, want refused naming alice", ok, who)
	}
}

// A ttl ends a hold: a crashed holder cannot wedge the rest.
func TestTTLExpiresTheHold(t *testing.T) {
	s := quick(t)
	s.Take(context.Background(), "g", "job", "alice@h", 150*time.Millisecond, 0)
	if ok, _ := s.Take(context.Background(), "g", "job", "bob@h", time.Minute, 0); ok {
		t.Fatal("the hold was still there before its ttl")
	}
	// The sweep fires within a few ticks of the expiry.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if ok, _ := s.Take(context.Background(), "g", "job", "bob@h", time.Minute, 0); ok {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the hold outlived its ttl by more than a second")
}

// Only the holder releases; --force releases anybody's, and both answer.
func TestReleaseRules(t *testing.T) {
	s := quick(t)
	s.Take(context.Background(), "g", "db", "alice@h", time.Minute, 0)
	if err := s.Release("g", "db", "bob@h", false); err == nil {
		t.Fatal("a non-holder released somebody else's lock")
	} else {
		var by *HeldBy
		if !errors.As(err, &by) || by.Holder != "alice@h" {
			t.Fatalf("the refusal did not name the holder: %v", err)
		}
	}
	if err := s.Release("g", "db", "alice@h", false); err != nil {
		t.Fatalf("the holder could not release: %v", err)
	}
	s.Take(context.Background(), "g", "db", "alice@h", time.Minute, 0)
	var displaced *Displaced
	if err := s.Release("g", "db", "bob@h", true); !errors.As(err, &displaced) || displaced.Previous != "alice@h" {
		t.Fatalf("force did not release another holder's lock naming who: %v", err)
	}
	if err := s.Release("g", "db", "bob@h", true); err != ErrNotHeld {
		t.Fatalf("releasing what nobody holds: %v", err)
	}
}

// Holders lists what the group holds, and only that group's.
func TestHoldersListsTheGroup(t *testing.T) {
	s := quick(t)
	s.Take(context.Background(), "ops", "a", "alice@h", time.Minute, 0)
	s.Take(context.Background(), "other", "a", "bob@h", time.Minute, 0)
	got := s.Holders("ops")
	if len(got) != 1 || got["a"].Holder != "alice@h" {
		t.Fatalf("holders(ops) = %v", got)
	}
	// Each hold carries its expiry, for the web face's time left.
	if left := time.Until(got["a"].Expires); left <= 50*time.Second || left > time.Minute {
		t.Fatalf("holders(ops) expiry is %v from now, want about a minute", left)
	}
}

// DropWhere forgets the holds on the records it names gone, and only those.
func TestDropWhereEndsOnlyTheGoneRecordsLocks(t *testing.T) {
	s := quick(t)
	s.Take(context.Background(), "ops", "a", "alice@h", time.Minute, 0)
	s.Take(context.Background(), "other", "b", "bob@h", time.Minute, 0)
	s.DropWhere(func(record string) bool { return record == "ops" })
	if got := s.Holders("ops"); len(got) != 0 {
		t.Fatalf("a hold survived its record's end: %v", got)
	}
	if got := s.Holders("other"); len(got) != 1 {
		t.Fatalf("an unrelated hold was dropped: %v", got)
	}
}

// A waiting take is granted when the holder releases, and names the holder
// while it waits.
func TestWaitEndsOnRelease(t *testing.T) {
	s := quick(t)
	s.Take(context.Background(), "g", "x", "alice@h", 10*time.Second, 0)
	granted := make(chan bool, 1)
	go func() {
		ok, _ := s.Take(context.Background(), "g", "x", "bob@h", time.Minute, 5*time.Second)
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
	s.Take(context.Background(), "g", "x", "alice@h", time.Minute, 0)
	start := time.Now()
	ok, who := s.Take(context.Background(), "g", "x", "bob@h", time.Minute, 200*time.Millisecond)
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
	s.Take(context.Background(), "g", "x", "alice@h", 250*time.Millisecond, 0)
	granted := make(chan bool, 1)
	go func() {
		ok, _ := s.Take(context.Background(), "g", "x", "bob@h", time.Minute, 3*time.Second)
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
	s.Take(context.Background(), "g", "x", "alice@h", 60*time.Millisecond, 0)
	time.Sleep(120 * time.Millisecond)
	if ok, who := s.Take(context.Background(), "g", "x", "bob@h", time.Minute, 0); !ok || who != "" {
		t.Fatalf("an expired hold refused a take without the sweep: %v %q", ok, who)
	}
}

// The sweep alone releases an expired hold nothing else touches.
func TestTheSweepAloneExpires(t *testing.T) {
	s := quickAt(t, 100*time.Millisecond)
	s.Take(context.Background(), "g", "x", "alice@h", 150*time.Millisecond, 0)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(s.Holders("g")) == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the sweep did not release the expired hold")
}

// A cancelled wait never grants: the lock may free after the caller gave up,
// but it is not theirs.
func TestCancelledWaitNeverGrants(t *testing.T) {
	s := quick(t)
	s.Take(context.Background(), "g", "x", "alice@h", 10*time.Second, 0)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() {
		ok, _ := s.Take(ctx, "g", "x", "bob@h", time.Minute, 10*time.Second)
		done <- ok
	}()
	time.Sleep(150 * time.Millisecond)
	cancel()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("a cancelled wait was granted")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a cancelled wait was not returned within 2s")
	}
	// The caller is gone, but the lock frees normally for the next one.
	if err := s.Release("g", "x", "alice@h", false); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.Take(context.Background(), "g", "x", "carol@h", time.Minute, 0); !ok {
		t.Fatal("the lock stayed wedged after the cancelled waiter")
	}
}

// A holder re-taking their own lock is refused at once, never waited on.
func TestSelfTakeRefusedAtOnce(t *testing.T) {
	s := quick(t)
	s.Take(context.Background(), "g", "x", "alice@h", 10*time.Second, 0)
	start := time.Now()
	ok, who := s.Take(context.Background(), "g", "x", "alice@h", time.Minute, 5*time.Second)
	if ok || who != "alice@h" {
		t.Fatalf("self-take: %v %q", ok, who)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("self-take waited %v rather than refusing at once", d)
	}
}

// Extend sets a fresh ttl from now, holder only.
func TestExtend(t *testing.T) {
	s := quick(t)
	s.Take(context.Background(), "g", "x", "alice@h", 150*time.Millisecond, 0)
	if err := s.Extend("g", "x", "alice@h", time.Minute); err != nil {
		t.Fatalf("the holder could not extend: %v", err)
	}
	// The old ttl no longer ends it.
	time.Sleep(250 * time.Millisecond)
	if got := s.Holders("g"); got["x"].Holder != "alice@h" {
		t.Fatalf("an extended hold expired on its old ttl: %v", got)
	}
	if err := s.Extend("g", "x", "bob@h", time.Minute); err == nil {
		t.Fatal("a non-holder extended somebody else's lock")
	}
	if err := s.Extend("g", "none", "alice@h", time.Minute); err != ErrNotHeld {
		t.Fatalf("extending what nobody holds: %v", err)
	}
}

// A waiter is woken when Holders purges an expired hold — not at the
// waiter's own deadline. The sweep is all but stopped, so Holders is the
// only path that could expire it.
func TestHoldersExpiryWakesTheWaiter(t *testing.T) {
	s := quick(t) // sweepEvery is 10s
	s.Take(context.Background(), "g", "x", "alice@h", 150*time.Millisecond, 0)
	granted := make(chan bool, 1)
	go func() {
		ok, _ := s.Take(context.Background(), "g", "x", "bob@h", time.Minute, 5*time.Second)
		granted <- ok
	}()
	time.Sleep(400 * time.Millisecond) // past the ttl, before the waiter's
	if got := s.Holders("g"); len(got) != 0 {
		t.Fatalf("Holders did not purge the expired hold: %v", got)
	}
	select {
	case ok := <-granted:
		if !ok {
			t.Fatal("the waiter was refused after Holders purged the hold")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the waiter was not woken by Holders purging the hold")
	}
}

// The same for Release's expired branch: a release that finds only an
// expired hold must wake the waiter, not answer ErrNotHeld and leave it.
func TestReleaseOfExpiredWakesTheWaiter(t *testing.T) {
	s := quick(t)
	s.Take(context.Background(), "g", "x", "alice@h", 150*time.Millisecond, 0)
	granted := make(chan bool, 1)
	go func() {
		ok, _ := s.Take(context.Background(), "g", "x", "bob@h", time.Minute, 5*time.Second)
		granted <- ok
	}()
	time.Sleep(400 * time.Millisecond)
	if err := s.Release("g", "x", "alice@h", false); err != ErrNotHeld {
		t.Fatalf("release of the expired hold: %v", err)
	}
	select {
	case ok := <-granted:
		if !ok {
			t.Fatal("the waiter was refused after Release expired the hold")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the waiter was not woken by Release expiring the hold")
	}
}

// All lists every hold, purged of what expired even before the sweep.
func TestAllPurgesExpiredHolds(t *testing.T) {
	saved := sweepEvery
	sweepEvery = time.Hour
	t.Cleanup(func() { sweepEvery = saved })
	s := New()
	t.Cleanup(s.Stop)
	s.Take(context.Background(), "ops", "brief", "alice@h", 10*time.Millisecond, 0)
	s.Take(context.Background(), "ops", "long", "alice@h", time.Minute, 0)
	time.Sleep(30 * time.Millisecond)
	got := s.All()
	if _, ok := got["ops"]["brief"]; ok || got["ops"]["long"].Holder != "alice@h" {
		t.Fatalf("All() = %v, want only the unexpired hold", got)
	}
}
