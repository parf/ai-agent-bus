package core

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A reader admitted through a Group loses its blocked read when the Group's
// membership or status changes through Manage, as through SetGroup.
func TestAGroupEditThroughManageReleasesItsReaders(t *testing.T) {
	for _, edit := range []string{"remove-member", "deactivate"} {
		t.Run(edit, func(t *testing.T) {
			b := New()
			b.SetDaemonOwner("admin@h")
			known(t, b, "alice@h", "bob@h")
			if err := b.SetGroup("alice@h", "@crew", []string{"bob@h"}); err != nil {
				t.Fatal(err)
			}
			provision(t, b, nil, protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h", Allow: []string{"@crew"}, Full: protocol.OverflowStrict})
			done := make(chan error, 1)
			go func() {
				_, err := b.ConsumeAs(context.Background(), "bob@h", "jobs@h", "", "", false, false)
				done <- err
			}()
			waitForWaiters(t, b, "jobs@h", 1)
			change := Management{Name: "@crew", Remove: &ListDelta{Allow: []string{"bob@h"}}}
			if edit == "deactivate" {
				change = Management{Name: "@crew", Status: ptr(protocol.StatusInactive)}
			}
			if _, err := b.Manage("alice@h", change); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if !errors.Is(err, ErrNotAllow) {
					t.Fatalf("the read ended for the wrong reason: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("a reader the group no longer admits is still blocked")
			}
		})
	}
}

// A Group's members are users, agents and groups; nothing else, and nothing
// that does not exist yet.
func TestAGroupsMembersAreActorsThatExist(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "#box@h")
	provision(t, b, nil,
		protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h", Full: protocol.OverflowStrict},
		protocol.Record{Name: "db@h", Kind: protocol.KindService, Owner: "alice@h", Addr: "db:1", Proto: "pg"},
	)
	for member, want := range map[string]error{"jobs@h": ErrBadName, "db@h": ErrBadName, "nobody@h": ErrUnknown} {
		if err := b.SetGroup("alice@h", "@crew", []string{member}); !errors.Is(err, want) {
			t.Errorf("%s as a member: %v, want %v", member, err, want)
		}
		if _, err := b.Manage("alice@h", Management{Name: "@team", Allow: &[]string{member}}); err == nil {
			t.Errorf("%s as a member through an allow list was accepted", member)
		}
	}
	if err := b.SetGroup("alice@h", "@crew", []string{"alice@h", "#box@h", "@later"}); err != nil {
		t.Fatalf("a user, an agent and a group reference: %v", err)
	}
}

// An Administrator edits a group's membership through either door.
func TestAnAdministratorEditsGroupsThroughManageToo(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "ops@h", "bob@h")
	if err := b.SetGroup("admin@h", AdministratorsGroup, []string{"admin@h", "ops@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("alice@h", "@crew", []string{"alice@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("ops@h", "@crew", []string{"alice@h", "bob@h"}); err != nil {
		t.Fatalf("an Administrator through SetGroup: %v", err)
	}
	if _, err := b.Manage("ops@h", Management{Name: "@crew", Remove: &ListDelta{Allow: []string{"bob@h"}}}); err != nil {
		t.Fatalf("an Administrator through Manage: %v", err)
	}
	if _, err := b.Manage("bob@h", Management{Name: "@crew", AddToSet: &ListDelta{Allow: []string{"bob@h"}}}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a non-member stranger edited the group: %v", err)
	}
}

// Inactive records are in no count, no owned list and no user view.
func TestInactiveRecordsAreInNoCountOrOwnedList(t *testing.T) {
	b := statusFixture(t)
	for _, n := range b.Owned("alice@h") {
		if n == "jobs@h" {
			t.Error("Owned lists an inactive record")
		}
	}
	if s := b.Status(); s.Kinds[protocol.KindQueue] != 0 {
		t.Errorf("Status counts an inactive queue: %d", s.Kinds[protocol.KindQueue])
	}
	for _, u := range b.Users("admin@h", nil) {
		for _, n := range u.Services {
			if n == "jobs@h" {
				t.Errorf("%s's view lists an inactive record", u.Name)
			}
		}
	}
}

// One send is one entry in the envelope feed, however many steps it takes.
func TestAForwardIntoATopicIsOneFeedEntry(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h")
	provision(t, b, nil,
		protocol.Record{Name: "#box@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"#box@h"}},
		protocol.Record{Name: "#a@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Subs: []string{"news@h"}, Full: protocol.OverflowStrict},
	)
	before := len(b.Recent("admin@h"))
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "#a@h", Body: "x"}); err != nil {
		t.Fatal(err)
	}
	if got := len(b.Recent("admin@h")) - before; got != 1 {
		t.Fatalf("one forwarded send is %d feed entries", got)
	}
}

// Ten topics deep is delivered; eleven is refused before anything is stored.
func TestAChainOfTopicsStopsAtTheForwardLimit(t *testing.T) {
	for _, depth := range []int{maxForwards, maxForwards + 1} {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			b := New()
			known(t, b, "alice@h")
			provision(t, b, nil, protocol.Record{Name: "#end@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict})
			// t0 → t1 → … → t{depth-1} → #end: depth forwarding steps.
			for i := depth - 1; i >= 0; i-- {
				next := fmt.Sprintf("t%d@h", i+1)
				if i == depth-1 {
					next = "#end@h"
				}
				provision(t, b, nil, protocol.Record{Name: fmt.Sprintf("t%d@h", i), Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}, Subs: []string{next}})
			}
			_, err := b.Send(protocol.Envelope{From: "alice@h", To: "t0@h", Body: "x"})
			q, _, _, _ := counts(b, "#end@h")
			if depth == maxForwards && (err != nil || q != 1) {
				t.Fatalf("ten steps: %v, %d stored", err, q)
			}
			if depth > maxForwards && (!errors.Is(err, ErrForwards) || q != 0) {
				t.Fatalf("eleven steps: %v, %d stored", err, q)
			}
		})
	}
}
