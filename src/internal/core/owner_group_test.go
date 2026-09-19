package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func ownerACLFixture(t *testing.T) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("root@h")
	for _, name := range []string{"alice@h", "bob@h", "stranger@h"} {
		if _, err := b.SetUser("root@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []protocol.Record{
		{Kind: protocol.KindAgent, Name: "target@h", Owner: "alice@h", Allow: []string{OwnerGroup}},
		{Kind: protocol.KindAgent, Name: "alice-service@h", Owner: "alice@h"},
		{Name: "alice-agent@h", Owner: "alice@h", Kind: "agent"},
		{Name: "alice-topic@h", Owner: "alice@h", Kind: protocol.KindQueue},
		{Kind: protocol.KindAgent, Name: "bob-service@h", Owner: "bob@h"},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "indirect@h", Owner: "alice-service@h"}); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestOwnerACLMeansDirectOwnerAndTheirServicesAndAgents(t *testing.T) {
	b := ownerACLFixture(t)
	for _, tc := range []struct {
		caller           string
		visible, allowed bool
	}{
		{"alice@h", true, true},
		{"alice-service@h", true, true},
		{"alice-agent@h", true, true},
		{"alice-topic@h", false, false},
		{"indirect@h", false, false},
		{"bob@h", false, false},
		{"bob-service@h", false, false},
		{"stranger@h", false, false},
		{"root@h", true, false},
	} {
		t.Run(tc.caller, func(t *testing.T) {
			_, visible := b.Lookup(tc.caller, "target@h")
			if visible != tc.visible {
				t.Fatalf("visibility=%v, want %v", visible, tc.visible)
			}
			_, err := b.Send(protocol.Envelope{From: tc.caller, To: "target@h", Body: tc.caller})
			if (err == nil) != tc.allowed {
				t.Fatalf("send=%v, allowed=%v", err, tc.allowed)
			}
		})
	}
	descr := "must stay owner-managed"
	if _, err := b.Manage("alice-service@h", Management{Name: "target@h", Descr: &descr}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("@owner access granted management: %v", err)
	}
}

func TestOwnerACLTracksDirectOwnershipChanges(t *testing.T) {
	b := ownerACLFixture(t)
	if _, err := b.Manage("alice@h", Management{Name: "target@h", Owner: ptr("bob@h")}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		caller  string
		allowed bool
	}{
		{"alice-service@h", false},
		{"bob@h", true},
		{"bob-service@h", true},
	} {
		if _, err := b.Send(protocol.Envelope{From: tc.caller, To: "target@h"}); (err == nil) != tc.allowed {
			t.Fatalf("after transfer %s: %v, allowed=%v", tc.caller, err, tc.allowed)
		}
	}
}

func TestOwnerACLIsNeverAStoredOrAuthorityGroup(t *testing.T) {
	b := ownerACLFixture(t)
	if err := b.SetGroup("root@h", OwnerGroup, []string{"alice@h"}); !errors.Is(err, ErrBadName) {
		t.Fatalf("created runtime group: %v", err)
	}
	if err := b.SetGroup("root@h", "@ordinary", []string{OwnerGroup}); !errors.Is(err, ErrBadName) {
		t.Fatalf("nested runtime group: %v", err)
	}
	maintainers := protocol.MaintainerList{OwnerGroup}
	if _, err := b.Manage("alice@h", Management{Name: "target@h", Maintainers: &maintainers}); !errors.Is(err, ErrBadName) {
		t.Fatalf("assigned runtime term as Maintainers: %v", err)
	}
	personal := true
	allow := []string{OwnerGroup}
	if _, err := b.Manage("alice@h", Management{Name: "target@h", Personal: &personal, Allow: &allow}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("personal service accepted runtime term: %v", err)
	}
}

func TestStoredOwnerGroupFailsClosed(t *testing.T) {
	b := New()
	b.Restore(ports.Snapshot{Groups: map[string][]string{OwnerGroup: {"alice@h"}}})
	if err := b.EstablishDaemonOwner("root@h"); err == nil || !strings.Contains(err.Error(), "runtime ACL term @owner") {
		t.Fatalf("stored runtime group did not fail closed: %v", err)
	}
}
