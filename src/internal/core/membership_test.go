package core

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// What a name leaves behind when it stops being a principal
// (docs/01-identity.md#unregistering).

func restored(t *testing.T, records ...protocol.Record) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("owner@h")
	for _, who := range []string{"active@h", "other@h", "paused@h", "banned@h"} {
		if _, err := b.SetUser("owner@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	b.Restore(ports.Snapshot{Clean: true, Records: records})
	return b
}

// The same rule from the other side, where it was live: an ordinary removal
// left the membership behind, and whoever took the freed name inherited it.
func TestAnUnregisteredNameKeepsNoGroupMembership(t *testing.T) {
	b := restored(t)
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "active@h", Kind: "generic"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("owner@h", "@ops", []string{"svc@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "secret@h", Owner: "active@h", Kind: "generic", Allow: []string{"@ops"}}); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("svc@h", "active@h"); err != nil {
		t.Fatal(err)
	}
	if members := b.Groups("owner@h")["@ops"]; len(members) != 0 {
		t.Fatalf("an unregistered name is still in @ops: %v", members)
	}
	// Somebody else takes the freed name. Without the strip above they arrive
	// already in @ops, and reach a record only @ops may reach.
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "other@h", Kind: "generic"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "svc@h", To: "secret@h", Body: "inherited"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("whoever took the freed name inherited its group: %v", err)
	}
}

// A person keeps their standing when a record of theirs goes: they are still a
// user, and being a maintainer is not something unregistering takes away.
func TestRemovingAPersonsRecordLeavesTheirMemberships(t *testing.T) {
	b := restored(t)
	if err := b.SetGroup("owner@h", "@ops", []string{"active@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("active@h", "active@h"); err != nil {
		t.Fatal(err)
	}
	if members := b.Groups("owner@h")["@ops"]; len(members) != 1 || members[0] != "active@h" {
		t.Fatalf("a person lost a membership by removing a record: %v", members)
	}
}
