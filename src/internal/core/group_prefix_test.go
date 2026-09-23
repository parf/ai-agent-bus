package core

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A prefixed group is its owner's: everything before the first "/" is a
// User's whole name, and the rest is the group's own name with any realm
// (docs/constitution.md#-group).
func TestAPrefixedGroupNameIsItsOwnersWholeName(t *testing.T) {
	for _, ok := range []string{"@alice/friends", "@alice@srv1/friends", "@alice@srv1/friends@batch1", "@a.b-c@srv1/x_y@r"} {
		if !groupName(ok) {
			t.Errorf("%s was refused as a group name", ok)
		}
	}
	for _, bad := range []string{"@/friends", "@alice/", "@alice@srv1/", "@#a/friends", "@alice/#friends", "@alice/x/y", "@alice@/x"} {
		if groupName(bad) {
			t.Errorf("%s was accepted as a group name", bad)
		}
	}
	if owner, ok := groupOwner("@alice@srv1/friends@batch1"); !ok || owner != "alice@srv1" {
		t.Errorf("groupOwner = %q, %v; want alice@srv1", owner, ok)
	}
	if _, ok := groupOwner("@ops@h"); ok {
		t.Error("an unprefixed group was given an owner by its name")
	}
}

func TestAPersonalGroupIsNamedForItsOwner(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@srv1", "bob@h")

	// The prefix reserves the name: bob cannot take alice's.
	if err := b.SetGroup("bob@h", "@alice@srv1/friends", []string{"bob@h"}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("bob created a group under alice's prefix: %v", err)
	}
	if err := b.SetGroup("alice@srv1", "@alice@srv1/friends@batch1", []string{"alice@srv1"}); err != nil {
		t.Fatalf("alice could not create her own prefixed group: %v", err)
	}
	on := true
	if _, err := b.Manage("alice@srv1", Management{Name: "@alice@srv1/friends@batch1", Personal: &on}); err != nil {
		t.Fatalf("alice could not make her prefixed group Personal: %v", err)
	}

	// Personal needs the prefix.
	if err := b.SetGroup("alice@srv1", "@crew", []string{"alice@srv1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@srv1", Management{Name: "@crew", Personal: &on}); !errors.Is(err, ErrKind) {
		t.Fatalf("an unprefixed group became Personal: %v", err)
	}
	if r, _ := b.Lookup("alice@srv1", "@crew"); r.Personal {
		t.Fatal("the refused Personal change was stored")
	}

	// A prefixed group cannot be transferred; the shared one can.
	bob := "bob@h"
	if _, err := b.Manage("alice@srv1", Management{Name: "@alice@srv1/friends@batch1", Owner: &bob}); !errors.Is(err, ErrKind) {
		t.Fatalf("a prefixed group was transferred: %v", err)
	}
	if r, _ := b.Lookup("alice@srv1", "@alice@srv1/friends@batch1"); r.Owner != "alice@srv1" {
		t.Fatalf("the refused transfer moved it to %s", r.Owner)
	}
	if _, err := b.Manage("alice@srv1", Management{Name: "@crew", Owner: &bob}); err != nil {
		t.Fatalf("a shared group could not be transferred: %v", err)
	}
}

// A database edited by hand into a mismatch is ignored and reported at load,
// like any incorrect record.
func TestAPrefixedGroupOwnedByAnotherIsIgnoredAtLoad(t *testing.T) {
	b := New()
	rep := &reports{}
	b.Journal(rep)
	b.Restore(withUsers(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "@alice/friends", Kind: protocol.KindGroup, Owner: "bob@h", Allow: []string{"bob@h"}},
		{Name: "@crew", Kind: protocol.KindGroup, Owner: "bob@h", Allow: []string{"bob@h"}, Personal: true},
		{Name: "@bob@h/mine", Kind: protocol.KindGroup, Owner: "bob@h", Allow: []string{"bob@h"}, Personal: true},
	}}, "alice", "bob@h"))
	for _, name := range []string{"@alice/friends", "@crew"} {
		if _, known := b.entity(name); known {
			t.Errorf("%s loaded although its name does not fit its owner", name)
		}
	}
	if _, known := b.entity("@bob@h/mine"); !known {
		t.Error("a correct prefixed Personal group was not loaded")
	}
}
