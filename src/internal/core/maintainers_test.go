package core

import "testing"

// The owner runs the maintainers group and cannot step out of it. Nothing
// covered these rules before, so each one here is the first thing that would
// notice them changing. See docs/01-identity.md#groups-and-maintainers.
func TestOwnerRunsTheMaintainersGroup(t *testing.T) {
	b := New()
	b.Administrator("owner@h")

	// Setup puts the owner in the group, so the list says what isMaintainer
	// already answers.
	if !b.member("owner@h", MaintainersGroup) {
		t.Fatal("the owner is not in the group it administers")
	}

	// Adding one.
	if err := b.SetGroup("owner@h", MaintainersGroup, []string{"owner@h", "second@h"}, false); err != nil {
		t.Fatalf("the owner could not add a maintainer: %v", err)
	}
	if !b.IsMaintainer("second@h") {
		t.Error("the added maintainer is not one")
	}

	// And taking them away again: add and delete are both the owner's.
	if err := b.SetGroup("owner@h", MaintainersGroup, []string{"owner@h"}, false); err != nil {
		t.Fatalf("the owner could not remove a maintainer: %v", err)
	}
	if b.IsMaintainer("second@h") {
		t.Error("a removed maintainer is still one")
	}

	// But not themselves. The check is about the list telling the truth:
	// isMaintainer answers yes for the owner whatever the list says, so a
	// list without them would describe a group they are in fact in.
	if err := b.SetGroup("owner@h", MaintainersGroup, []string{"second@h"}, false); err == nil {
		t.Error("the owner removed themselves from the maintainers group")
	}

	// And the group itself is never deleted, by anybody.
	if err := b.SetGroup("owner@h", MaintainersGroup, nil, true); err == nil {
		t.Error("the maintainers group was deleted")
	}

	// A maintainer who is not the owner may not touch this one group, or they
	// could promote themselves through it.
	if err := b.SetGroup("owner@h", MaintainersGroup, []string{"owner@h", "second@h"}, false); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("second@h", MaintainersGroup, []string{"owner@h", "second@h", "third@h"}, false); err == nil {
		t.Error("a maintainer edited the group that defines their own level")
	}
	// Positive control: that same maintainer may edit an ordinary group, so
	// the refusal above is about which group and not about who asked.
	if err := b.SetGroup("second@h", "@ops", []string{"third@h"}, false); err != nil {
		t.Errorf("a maintainer could not edit an ordinary group: %v", err)
	}
}

// Administrator writes a profile for the owner, which is why the owner is a
// registered user rather than a credential with nothing behind it. The
// ownerless-credential sweep depends on this being true.
// See docs/02-access.md#ownerless-credentials.
func TestTheOwnerIsARegisteredUser(t *testing.T) {
	b := New()
	if b.IsPerson("owner@h") {
		t.Fatal("a person before anybody said so")
	}
	b.Administrator("owner@h")
	if !b.IsPerson("owner@h") {
		t.Error("the daemon owner is not a registered user, so a sweep keyed on that would take its credential")
	}
}
