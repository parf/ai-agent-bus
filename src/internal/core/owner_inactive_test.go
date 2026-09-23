package core

import (
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The owner-inactive count is what reactivating a User would bring back: its
// records whose own status is active, and the messages they hold. Not the
// User's own record, not a record inactive in its own right, not an active
// owner's backlog (docs/05-discovery.md#overview-and-diagnostics).
func TestOwnerInactiveCountsOnlyRecordsInactiveThroughTheirOwner(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	for _, name := range []string{"alice@h", "bob@h", "helper@h", "plain@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.SetGroup("admin@h", AdministratorsGroup, []string{"admin@h", "helper@h"}); err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{
		{Name: "#a1@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
		{Name: "#a2@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
		{Name: "#off@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
		{Name: "#b1@h", Kind: protocol.KindAgent, Owner: "bob@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	send := func(to string, n int) {
		t.Helper()
		from := "admin@h"
		if to == "alice@h" {
			from = "#a1@h" // a User takes messages from its own agents
		}
		for i := 0; i < n; i++ {
			if _, err := b.Send(protocol.Envelope{From: from, To: to, Body: "held"}); err != nil {
				t.Fatalf("send to %s: %v", to, err)
			}
		}
	}
	send("#a1@h", 2)
	send("#a2@h", 1)
	send("#off@h", 4) // inactive in its own right: stays inactive on reactivation
	send("#b1@h", 8)  // an active owner's backlog is ordinary work
	send("alice@h", 16)
	if _, err := b.Manage("alice@h", Management{Name: "#off@h", Status: ptr(protocol.StatusInactive)}); err != nil {
		t.Fatal(err)
	}

	// Before anybody is inactive the count is present and zero: observed none.
	if got := b.StatusFor("admin@h").OwnerInactive; got == nil || *got != (OwnerInactive{}) {
		t.Fatalf("before any User is inactive: %+v, want present and zero", got)
	}
	if _, err := b.SetUserState("admin@h", "alice@h", protocol.StatusInactive); err != nil {
		t.Fatal(err)
	}
	want := OwnerInactive{Records: 2, Messages: 3}
	for _, who := range []string{"admin@h", "helper@h"} {
		if got := b.StatusFor(who).OwnerInactive; got == nil || *got != want {
			t.Errorf("%s reads %+v, want %+v", who, got, want)
		}
	}
	// Node-wide, so nobody else is answered it — not even the owning User's
	// peers, who would otherwise learn that somebody was made inactive.
	for _, who := range []string{"bob@h", "plain@h", "#b1@h"} {
		if got := b.StatusFor(who).OwnerInactive; got != nil {
			t.Errorf("%s is answered the node-wide count: %+v", who, *got)
		}
	}
	if b.Status().OwnerInactive != nil {
		t.Error("the callerless Status carries the count")
	}

	if _, err := b.SetUserState("admin@h", "alice@h", protocol.StatusActive); err != nil {
		t.Fatal(err)
	}
	if got := b.StatusFor("admin@h").OwnerInactive; got == nil || *got != (OwnerInactive{}) {
		t.Errorf("after reactivation: %+v, want zero", got)
	}
}
