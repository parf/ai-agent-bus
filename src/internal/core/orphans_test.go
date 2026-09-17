package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// H.5.5. A record whose owner the daemon knows nothing about is wreckage and
// goes at startup with everything that hung on it
// (docs/01-identity.md#when-the-owner-is-gone). The end-to-end start over a
// store on disk is the smoke case "a start clears out the records whose owner it
// does not know"; these are the semantics a start cannot present — a fixed
// point, a blocked reader, and the guards an ordinary removal applies.

func wreck(name, owner string) protocol.Record {
	return protocol.Record{Name: name, Owner: owner, Kind: "generic", Full: protocol.OverflowStrict}
}

func listed(t *testing.T, b *Bus, name string) bool {
	t.Helper()
	_, ok := b.Lookup("owner@h", name)
	return ok
}

// A stopped owner is not a missing one. These three controls are the whole
// reason the predicate asks what the daemon *knows* about a name rather than
// what it is currently willing to let that name do.
func TestAStoppedOwnerIsNotAMissingOne(t *testing.T) {
	b := restored(t,
		wreck("wreck@h", "nobody@h"),
		wreck("live@h", "active@h"),
		wreck("paused-svc@h", "paused@h"),
		wreck("banned-svc@h", "banned@h"),
	)
	controls := []string{"live@h", "paused-svc@h", "banned-svc@h"}
	for _, name := range controls {
		if _, err := b.Send(protocol.Envelope{From: "owner@h", To: name, Body: "kept"}); err != nil {
			t.Fatalf("seeding %s: %v", name, err)
		}
	}
	// Suspended after the queues exist, because a suspended owner's service
	// refuses delivery (H.5.7) and there would otherwise be nothing to lose.
	for _, s := range []struct{ who, state string }{{"paused@h", "paused"}, {"banned@h", "banned"}} {
		if _, err := b.SetUserState("owner@h", s.who, s.state); err != nil {
			t.Fatal(err)
		}
	}

	if purged := b.Orphans(); len(purged) != 1 || purged[0] != "wreck@h" {
		t.Fatalf("purged %v, want only wreck@h", purged)
	}
	for _, name := range controls {
		rec, ok := b.Lookup("owner@h", name)
		if !ok {
			t.Errorf("%s was deleted; its owner is stopped, not gone", name)
			continue
		}
		if rec.Queued != 1 {
			t.Errorf("%s survived but lost its queue: %d held", name, rec.Queued)
		}
	}
}

// Deleting makes orphans, so the sweep is not a pass. A owns B and B owns C,
// all three records: taking A is what makes B unknown, and taking B is what
// makes C unknown. One pass leaves C live.
func TestDeletionRunsToAFixedPoint(t *testing.T) {
	b := restored(t,
		wreck("a@h", "nobody@h"),
		wreck("b@h", "a@h"),
		wreck("c@h", "b@h"),
		wreck("live@h", "active@h"),
	)
	purged := b.Orphans()
	if len(purged) != 3 {
		t.Fatalf("purged %v, want a@h b@h c@h", purged)
	}
	for _, name := range []string{"a@h", "b@h", "c@h"} {
		if listed(t, b, name) {
			t.Errorf("%s survived; the sweep stopped before its own fixed point", name)
		}
	}
	if !listed(t, b, "live@h") {
		t.Error("the cascade reached a record whose owner the daemon knows")
	}
}

// The freed name is reserved to nobody, exactly as unregistering leaves it,
// and what was queued for the old one is not waiting for the new one.
func TestAPurgedNameIsFreeAndItsQueueIsGone(t *testing.T) {
	b := restored(t, wreck("wreck@h", "nobody@h"))
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "wreck@h", Body: "addressed to nobody"}); err != nil {
		t.Fatal(err)
	}
	b.Orphans()

	if _, err := b.Register(protocol.Record{Name: "wreck@h", Owner: "active@h", Kind: "generic"}); err != nil {
		t.Fatalf("the freed name did not register to somebody else: %v", err)
	}
	rec, _ := b.Lookup("owner@h", "wreck@h")
	if rec.Owner != "active@h" {
		t.Errorf("the name came back owned by %q", rec.Owner)
	}
	if rec.Queued != 0 {
		t.Errorf("the new owner inherited %d of the purged messages", rec.Queued)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if e, err := b.ConsumeAs(ctx, "active@h", "wreck@h", "", "", false, false); err == nil {
		t.Errorf("the new owner was handed purged work: %q", e.Body)
	}
}

// A blocked reader cannot exist at startup — a fresh process has no waiters —
// so the release is exercised here, directly, with one attached. The reader is
// filtered and the queued work does not match it; an unfiltered one would take
// the message rather than block, and there would be nothing to release.
func TestAPurgeReleasesItsBlockedReaderWithNothingFromTheQueue(t *testing.T) {
	b := restored(t, wreck("wreck@h", "nobody@h"))
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "wreck@h", Topic: "other", Body: "purged"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type read struct {
		e   protocol.Envelope
		err error
	}
	result := make(chan read, 1)
	go func() {
		e, err := b.ConsumeAs(ctx, "active@h", "wreck@h", "mine", "", true, false)
		result <- read{e, err}
	}()
	deadline := time.Now().Add(2 * time.Second)
	for b.Status().Waiting != 1 {
		if time.Now().After(deadline) {
			t.Fatal("the reader never blocked, so the release is not being tested")
		}
		time.Sleep(time.Millisecond)
	}

	b.Orphans()

	select {
	case got := <-result:
		if got.err == nil {
			t.Fatalf("the released reader was handed purged work: %q", got.e.Body)
		}
		if !errors.Is(got.err, ErrUnknown) {
			t.Errorf("released with %v, want no such name", got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the reader was never released; its inbox went out from under it")
	}
}

// Both of UnregisterAnd's refusals are bypassed, so both need a case. They
// exist because there is somebody to tell and the choice is theirs; for
// wreckage there is nobody, and leaving it is the worse answer.
func TestAPurgeAsksNeitherGuardThatUnregisterAsks(t *testing.T) {
	b := restored(t,
		wreck("owns@h", "nobody@h"), wreck("kid@h", "owns@h"),
		wreck("queued@h", "nobody@h"),
		// The same two shapes with an owner the daemon knows, to show by hand
		// that each guard does refuse what the purge below walks past.
		wreck("ctrl-owns@h", "active@h"), wreck("ctrl-kid@h", "ctrl-owns@h"),
		wreck("ctrl-queued@h", "active@h"),
	)
	for _, name := range []string{"queued@h", "ctrl-queued@h"} {
		if _, err := b.Send(protocol.Envelope{From: "owner@h", To: name, Body: "in the way"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct{ name, guard string }{
		{"ctrl-owns@h", "still owns"},
		{"ctrl-queued@h", "queued messages"},
	} {
		err := b.Unregister(c.name, "active@h")
		if !errors.Is(err, ErrBusy) || !strings.Contains(err.Error(), c.guard) {
			t.Fatalf("by-hand removal of %s did not refuse with %q: %v", c.name, c.guard, err)
		}
	}

	purged := b.Orphans()
	if len(purged) != 3 {
		t.Fatalf("purged %v, want owns@h kid@h queued@h", purged)
	}
	for _, name := range []string{"owns@h", "kid@h", "queued@h"} {
		if listed(t, b, name) {
			t.Errorf("%s survived the purge; it inherited a guard from the by-hand path", name)
		}
	}
	// The controls are untouched, so the guards above still hold for a name
	// with an owner to refuse on behalf of.
	for _, name := range []string{"ctrl-owns@h", "ctrl-kid@h", "ctrl-queued@h"} {
		if !listed(t, b, name) {
			t.Errorf("%s went with the wreckage", name)
		}
	}
}

// A registered user's own record is never wreckage, whoever the store says
// owns it. The person is the principal that answers for the name, so the
// reason the rule gives for taking a record does not hold — and taking it took
// their inbox, their memberships and, by handing the name back for collection,
// their credential.
func TestARegisteredUsersOwnRecordIsNotWreckage(t *testing.T) {
	b := restored(t,
		protocol.Record{Name: "active@h", Owner: "missing@h", Kind: "agent", Full: protocol.OverflowStrict},
		wreck("svc@h", "missing@h"),
	)
	if err := b.SetGroup("owner@h", "@ops", []string{"active@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "active@h", Body: "for a real person"}); err != nil {
		t.Fatal(err)
	}

	// The service beside it is the control: the same missing owner, and it
	// goes. So this is about the person, not about the fixture.
	if purged := b.Orphans(); len(purged) != 1 || purged[0] != "svc@h" {
		t.Fatalf("purged %v, want only svc@h", purged)
	}
	rec, ok := b.Lookup("owner@h", "active@h")
	if !ok {
		t.Fatal("a registered user's record was taken as wreckage")
	}
	if rec.Queued != 1 {
		t.Errorf("their queue went with it: %d held", rec.Queued)
	}
	if members := b.Groups("owner@h")["@ops"]; len(members) != 1 {
		t.Errorf("their group membership went with it: %v", members)
	}
	// And the sweep that decides credentials agrees, which is the half that
	// would have dropped the token: it spares a profile for the same reason.
	if swept := b.Ownerless([]string{"active@h"}); len(swept) != 0 {
		t.Errorf("their credential was collected: %v", swept)
	}
	// They can still clear it up themselves — manages allows the name itself,
	// so an unreachable owner does not strand them with a record nobody can
	// touch. The refusal they get first is the queue guard, which is the
	// ordinary one and is theirs to answer.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := b.Unregister("active@h", "active@h"); !errors.Is(err, ErrBusy) {
		t.Errorf("removing their own record was refused as %v, not as a busy queue", err)
	}
	if _, err := b.ConsumeAs(ctx, "active@h", "active@h", "", "", false, false); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("active@h", "active@h"); err != nil {
		t.Errorf("they cannot remove their own record: %v", err)
	}
}

// The waits a purged name left on inboxes that SURVIVED. Its own inbox is
// gone; these are somewhere else, and nothing else collects them — after the
// delete there is no inbox for recheckReaders to find it through.
func TestAPurgeReleasesTheWaitsItLeftOnOtherInboxes(t *testing.T) {
	b := restored(t, wreck("gone@h", "nobody@h"), wreck("kept@h", "active@h"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got := make(chan protocol.Envelope, 1)
	go func() {
		defer close(got)
		if e, err := b.ConsumeAs(ctx, "gone@h", "kept@h", "", "", false, true); err == nil {
			got <- e
		}
	}()
	deadline := time.Now().Add(2 * time.Second)
	for b.Status().Waiting != 1 {
		if time.Now().After(deadline) {
			t.Fatal("the reader never blocked on the other inbox")
		}
		time.Sleep(time.Millisecond)
	}

	b.Orphans()

	if w := b.Status().Waiting; w != 0 {
		t.Errorf("a name the daemon no longer knows is still waiting on a live inbox: %d", w)
	}
	// The consequence, not just the count: deliver trusts the waiters it
	// finds attached and does not ask the ACL again.
	if _, err := b.Send(protocol.Envelope{From: "active@h", To: "kept@h", Body: "post-purge"}); err != nil {
		t.Fatal(err)
	}
	select {
	case e, open := <-got:
		if open {
			t.Fatalf("a purged name was handed %q from an inbox that survived", e.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the reader was neither released nor served")
	}
}

// A cycle survives, by the rule rather than as an exception: each member's
// owner has a record, so each is known. Pinned because it is a decision, and a
// sweep that walked ownership to a person would take all three.
func TestAnOwnershipCycleSurvives(t *testing.T) {
	b := restored(t,
		wreck("x@h", "z@h"), wreck("y@h", "x@h"), wreck("z@h", "y@h"),
		wreck("under@h", "z@h"),
		wreck("gone@h", "nobody@h"),
	)
	if purged := b.Orphans(); len(purged) != 1 || purged[0] != "gone@h" {
		t.Fatalf("purged %v, want only gone@h", purged)
	}
	for _, name := range []string{"x@h", "y@h", "z@h", "under@h"} {
		if !listed(t, b, name) {
			t.Errorf("%s was taken; its owner has a record", name)
		}
	}
}

// A name the store lists as a maintainer comes back as a user, because the
// levels are nested and Restore rebuilds the profile from the group. So its
// records are not wreckage. It is only reachable by hand — SetGroup makes a
// maintainer a user at the moment of the call — and it is the same mechanism
// that gives the daemon owner their standing on a restart.
func TestAMaintainerInTheStoreIsAUser(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")
	b.Restore(ports.Snapshot{
		Clean:   true,
		Groups:  map[string][]string{AdministratorsGroup: {"owner@h", "vouched@h"}},
		Records: []protocol.Record{wreck("theirs@h", "vouched@h")},
	})
	if k := b.identityKind("vouched@h"); k != protocol.DirectoryUser {
		t.Fatalf("a restored maintainer is %q, not a user", k)
	}
	if purged := b.Orphans(); len(purged) != 0 {
		t.Errorf("a restored maintainer's records were taken as wreckage: %v", purged)
	}
}

// A freed name is reclaimable by anybody, so a membership left behind is
// inherited rather than merely stale.
func TestAPurgedNameKeepsNoGroupMembership(t *testing.T) {
	b := restored(t, wreck("wreck@h", "nobody@h"))
	if err := b.SetGroup("owner@h", "@ops", []string{"wreck@h"}); err != nil {
		t.Fatal(err)
	}
	b.Orphans()
	if members := b.Groups("owner@h")["@ops"]; len(members) != 0 {
		t.Fatalf("the purged name is still in @ops: %v", members)
	}
}
