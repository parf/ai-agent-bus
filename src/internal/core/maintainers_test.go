package core

import (
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
)

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
	if err := b.SetGroup("owner@h", MaintainersGroup, []string{"owner@h", "second@h"}); err != nil {
		t.Fatalf("the owner could not add a maintainer: %v", err)
	}
	if !b.IsMaintainer("second@h") {
		t.Error("the added maintainer is not one")
	}

	// And taking them away again: add and delete are both the owner's.
	if err := b.SetGroup("owner@h", MaintainersGroup, []string{"owner@h"}); err != nil {
		t.Fatalf("the owner could not remove a maintainer: %v", err)
	}
	if b.IsMaintainer("second@h") {
		t.Error("a removed maintainer is still one")
	}

	// But not themselves. The check is about the list telling the truth:
	// isMaintainer answers yes for the owner whatever the list says, so a
	// list without them would describe a group they are in fact in.
	if err := b.SetGroup("owner@h", MaintainersGroup, []string{"second@h"}); err == nil {
		t.Error("the owner removed themselves from the maintainers group")
	}

	// A maintainer who is not the owner may not touch this one group, or they
	// could promote themselves through it.
	if err := b.SetGroup("owner@h", MaintainersGroup, []string{"owner@h", "second@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("second@h", MaintainersGroup, []string{"owner@h", "second@h", "third@h"}); err == nil {
		t.Error("a maintainer edited the group that defines their own level")
	}
	// Positive control: that same maintainer may edit an ordinary group, so
	// the refusal above is about which group and not about who asked.
	if err := b.SetGroup("second@h", "@ops", []string{"third@h"}); err != nil {
		t.Errorf("a maintainer could not edit an ordinary group: %v", err)
	}
}

// The three levels are nested, not side by side: an owner is a maintainer and
// a maintainer is a user. A maintainer who was not a user would be a principal
// with authority over users that user administration could not see, and a
// credential the ownerless sweep would take.
// See docs/01-identity.md#groups-and-maintainers.
func TestTheLevelsAreNested(t *testing.T) {
	b := New()
	if b.IsPerson("owner@h") {
		t.Fatal("a person before anybody said so")
	}
	b.Administrator("owner@h")
	if !b.IsMaintainer("owner@h") {
		t.Error("the owner is not a maintainer")
	}
	if !b.IsPerson("owner@h") {
		t.Error("the owner is not a registered user, so a sweep keyed on that would take its credential")
	}

	if err := b.SetGroup("owner@h", MaintainersGroup, []string{"owner@h", "second@h"}); err != nil {
		t.Fatal(err)
	}
	if !b.IsMaintainer("second@h") {
		t.Error("an added maintainer is not a maintainer")
	}
	if !b.IsPerson("second@h") {
		t.Error("an added maintainer is not a registered user")
	}

	// Taking them out again leaves the person behind: users are not deleted,
	// only made inactive (docs/01-identity.md#user-lifecycle).
	if err := b.SetGroup("owner@h", MaintainersGroup, []string{"owner@h"}); err != nil {
		t.Fatal(err)
	}
	if b.IsMaintainer("second@h") {
		t.Error("a removed maintainer is still one")
	}
	if !b.IsPerson("second@h") {
		t.Error("removing a maintainer deleted the person")
	}

	// A group that is not the maintainers group confers nothing, so joining it
	// is not what makes somebody a user.
	if err := b.SetGroup("owner@h", "@ops", []string{"third@h"}); err != nil {
		t.Fatal(err)
	}
	if b.IsPerson("third@h") {
		t.Error("an ordinary group membership made a user, so the check above proves nothing")
	}
}

// A snapshot written before the rule can hold a maintainer with no profile.
// Restoring one must not carry the gap forward.
func TestRestoreMakesOldMaintainersUsers(t *testing.T) {
	b := New()
	b.Administrator("owner@h")
	b.Restore(ports.Snapshot{Groups: map[string][]string{
		MaintainersGroup: {"owner@h", "legacy@h"},
	}})
	if !b.IsMaintainer("legacy@h") {
		t.Fatal("the restored maintainer is not one")
	}
	if !b.IsPerson("legacy@h") {
		t.Error("a maintainer restored from an older snapshot is still not a user")
	}
}
