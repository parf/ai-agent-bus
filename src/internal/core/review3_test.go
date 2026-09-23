package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func blockedRead(t *testing.T, b *Bus, caller, inbox string) chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := b.ConsumeAs(context.Background(), caller, inbox, "", "", false, false)
		done <- err
	}()
	waitForWaiters(t, b, inbox, 1)
	return done
}

func released(t *testing.T, done chan error, what string) {
	t.Helper()
	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("%s: the read was handed a message", what)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("%s: a reader that lost authority is still blocked", what)
	}
}

// Deactivating an agent releases its reads elsewhere, and a transfer that
// takes it off its old Owner's Personal records releases its reads there.
func TestReadersLoseTheirReadsWhenTheirStandingGoes(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h")
	provision(t, b, nil,
		protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Full: protocol.OverflowStrict},
		protocol.Record{Name: "q@h", Kind: protocol.KindQueue, Owner: "alice@h", Allow: []string{"#a@h"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "pq@h", Kind: protocol.KindQueue, Owner: "alice@h", Personal: true, Allow: []string{"#a@h"}, Full: protocol.OverflowStrict},
	)
	done := blockedRead(t, b, "#a@h", "q@h")
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Status: ptr(protocol.StatusInactive)}); err != nil {
		t.Fatal(err)
	}
	released(t, done, "deactivation")
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Status: ptr(protocol.StatusActive)}); err != nil {
		t.Fatal(err)
	}
	done = blockedRead(t, b, "#a@h", "pq@h")
	bob := "bob@h"
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Owner: &bob}); err != nil {
		t.Fatal(err)
	}
	released(t, done, "transfer")
	if _, err := b.Send(protocol.Envelope{From: "#a@h", To: "pq@h", Body: "private"}); err == nil {
		t.Fatal("the transferred agent still writes to the old owner's Personal record")
	}
}

// Even a reader some edit forgot to release is never handed a message it may
// no longer read: delivery asks at the hand-over.
func TestDeliveryAsksAtTheHandOver(t *testing.T) {
	b := New()
	known(t, b, "alice@h", "bob@h")
	provision(t, b, nil, protocol.Record{Name: "q@h", Kind: protocol.KindQueue, Owner: "alice@h", Allow: []string{"bob@h", "alice@h"}, Full: protocol.OverflowStrict})
	done := blockedRead(t, b, "bob@h", "q@h")
	b.mu.Lock()
	r := b.records["q@h"]
	r.Allow = []string{"alice@h"}
	b.records["q@h"] = r // an edit that did not recheck its readers
	b.mu.Unlock()
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "q@h", Body: "not bob's"}); err != nil {
		t.Fatal(err)
	}
	released(t, done, "hand-over")
	if in := b.inboxes["q@h"]; in == nil || len(in.queue) != 1 {
		t.Fatal("the message the stale reader was refused is not queued")
	}
}

// A user record is made with its User and carries no lists; registration can
// neither squat one nor give one an ACL, and what is written loads back.
func TestAUserRecordIsItsUsersAndCarriesNoLists(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "mallory@h")
	if _, err := b.Register(protocol.Record{Name: "victim@h", Kind: protocol.KindUser, Owner: "alice@h"}); !errors.Is(err, ErrKind) {
		t.Fatalf("a user record was squatted for a name that is no User: %v", err)
	}
	if _, err := b.SetUser("admin@h", protocol.User{Name: "victim@h"}, true); err != nil {
		t.Fatalf("the refused squat still blocked the User: %v", err)
	}
	for what, change := range map[string]Management{
		"allow":       {Name: "alice@h", Allow: &[]string{"mallory@h"}},
		"maintainers": {Name: "alice@h", Maintainers: ptr(protocol.MaintainerList{"mallory@h"})},
	} {
		if _, err := b.Manage("alice@h", change); !errors.Is(err, ErrKind) {
			t.Errorf("a user record took %s: %v", what, err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := b.ConsumeAs(ctx, "mallory@h", "alice@h", "", "", false, false); err == nil {
		t.Fatal("another user read alice's inbox")
	}
	rep := &reports{}
	c := New()
	c.Journal(rep)
	c.Restore(b.Snapshot())
	if rep.has("is ignored") {
		t.Fatalf("a restore ignored what this version wrote: %v", rep.lines)
	}
}

// "*" admits every active User and Agent, and no queue or topic as a
// forwarding source.
func TestTheWildcardAdmitsNoChannelAsASource(t *testing.T) {
	b := New()
	known(t, b, "alice@h")
	provision(t, b, nil,
		protocol.Record{Name: "#b@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h", Full: protocol.OverflowStrict},
		protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Full: protocol.OverflowStrict},
	)
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Subs: &[]string{"#b@h"}}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("\"*\" admitted a queue as a route source: %v", err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Subs: &[]string{"#b@h"}}); err != nil {
		t.Fatalf("\"*\" did not admit an agent source: %v", err)
	}
}

// An ignored record keeps its credential for the operator who repairs it.
func TestAnIgnoredRecordKeepsItsCredential(t *testing.T) {
	b := New()
	b.Journal(&reports{})
	b.Restore(withUsers(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "#broken@h", Kind: protocol.KindAgent, Owner: "nobody@h", Full: protocol.OverflowStrict},
	}}, "owner@h"))
	if got := b.Ownerless([]string{"#broken@h", "#never@h"}); len(got) != 1 || got[0] != "#never@h" {
		t.Fatalf("the ownerless sweep would take %v; an ignored record's credential stays", got)
	}
}

// A local account maps to a User or an Agent, and goes with the name when it
// is removed, so a reused name inherits no socket.
func TestAnAccountMapsToAnActorAndGoesWithIt(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h")
	provision(t, b, nil,
		protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Full: protocol.OverflowStrict},
		protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h", Full: protocol.OverflowStrict},
	)
	if _, err := b.SetAccount("admin@h", "ops", "jobs@h", false); !errors.Is(err, ErrKind) {
		t.Fatalf("a queue became a socket principal: %v", err)
	}
	if _, err := b.SetAccount("admin@h", "worker", "#a@h", false); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("#a@h", "alice@h"); err != nil {
		t.Fatal(err)
	}
	if got := b.accounts["worker"]; got != "" {
		t.Fatalf("the removed agent's account mapping survived: %s", got)
	}
}

// A name registered afresh inherits no stored credential from an ignored
// record that held it: the birth drops it in the same commit.
func TestABornRecordInheritsNoStoredCredential(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	idx := newIndex()
	idx.held["#lost@h"] = ports.CredentialPair{UserID: 999, AgentID: 999}
	b.BindCredentials(idx)
	known(t, b, "alice@h")
	if _, err := b.Register(protocol.Record{Name: "#lost@h", Kind: protocol.KindAgent, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	if _, still := idx.held["#lost@h"]; still {
		t.Fatal("the new #lost@h kept the old credential under its name")
	}
}
