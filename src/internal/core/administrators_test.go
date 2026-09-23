package core

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The owner runs the administrators group and cannot step out of it. Nothing
// covered these rules before, so each one here is the first thing that would
// notice them changing. See docs/01-identity-and-roles.md#groups.
func TestOwnerRunsTheAdministratorsGroup(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")

	// Setup puts the owner in the group, so the list says what isAdministrator
	// already answers.
	if !b.member("owner@h", AdministratorsGroup) {
		t.Fatal("the owner is not in the group it administers")
	}

	// Adding one.
	// An administrator is a User first (docs/01-identity-and-roles.md#daemon-administrators).
	if _, err := b.SetUser("owner@h", protocol.User{Name: "second@h"}, true); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h", "second@h"}); err != nil {
		t.Fatalf("the owner could not add a administrator: %v", err)
	}
	if !b.IsAdministrator("second@h") {
		t.Error("the added administrator is not one")
	}

	// And taking them away again: add and delete are both the owner's.
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h"}); err != nil {
		t.Fatalf("the owner could not remove a administrator: %v", err)
	}
	if b.IsAdministrator("second@h") {
		t.Error("a removed administrator is still one")
	}

	// But not themselves. The check is about the list telling the truth:
	// isAdministrator answers yes for the owner whatever the list says, so a
	// list without them would describe a group they are in fact in.
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"second@h"}); err == nil {
		t.Error("the owner removed themselves from the administrators group")
	}

	// A administrator who is not the owner may not touch this one group, or they
	// could promote themselves through it.
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h", "second@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("second@h", AdministratorsGroup, []string{"owner@h", "second@h", "third@h"}); err == nil {
		t.Error("a administrator edited the group that defines their own level")
	}
	// Positive control: that same administrator may edit an ordinary group, so
	// the refusal above is about which group and not about who asked.
	if _, err := b.SetUser("owner@h", protocol.User{Name: "third@h"}, true); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("second@h", "@ops", []string{"third@h"}); err != nil {
		t.Errorf("a administrator could not edit an ordinary group: %v", err)
	}
}

// The three levels are nested, not side by side: an owner is a administrator and
// a administrator is a user. A administrator who was not a user would be a principal
// with authority over users that user administration could not see, and a
// credential the ownerless sweep would take.
// See docs/01-identity-and-roles.md#groups.
func TestTheLevelsAreNested(t *testing.T) {
	b := New()
	if b.IsPerson("owner@h") {
		t.Fatal("a person before anybody said so")
	}
	b.SetDaemonOwner("owner@h")
	if !b.IsAdministrator("owner@h") {
		t.Error("the owner is not a administrator")
	}
	if !b.IsPerson("owner@h") {
		t.Error("the owner is not a registered user, so a sweep keyed on that would take its credential")
	}

	// An administrator is made from a User, never the other way round: the
	// protected group takes Users only.
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h", "ghost@h"}); !errors.Is(err, ErrBadName) || b.IsPerson("ghost@h") {
		t.Fatalf("an unknown name was made an administrator, or a User manufactured for it: %v", err)
	}
	if _, err := b.SetUser("owner@h", protocol.User{Name: "second@h"}, true); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h", "second@h"}); err != nil {
		t.Fatal(err)
	}
	if !b.IsAdministrator("second@h") {
		t.Error("an added administrator is not a administrator")
	}
	if !b.IsPerson("second@h") {
		t.Error("an added administrator is not a registered user")
	}

	// Taking them out again leaves the person behind: users are not deleted,
	// only made inactive (docs/01-identity-and-roles.md#user-states).
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h"}); err != nil {
		t.Fatal(err)
	}
	if b.IsAdministrator("second@h") {
		t.Error("a removed administrator is still one")
	}
	if !b.IsPerson("second@h") {
		t.Error("removing a administrator deleted the person")
	}

	// A group that is not the administrators group confers nothing, so joining it
	// is not what makes somebody a user.
	// An ordinary group takes actors that exist and makes nobody a user.
	if err := b.SetGroup("owner@h", "@ops", []string{"third@h"}); !errors.Is(err, ErrUnknown) || b.IsPerson("third@h") {
		t.Errorf("an ordinary group took an unknown member, or made it a user: %v", err)
	}
}

// An @administrators line naming no User is one no write of this version
// made. It is ignored and reported, not repaired: no User, no record and no ID
// are manufactured for it (docs/constitution.md#persistence-and-loading).
func TestRestoreIgnoresAnAdministratorWhoIsNoUser(t *testing.T) {
	b := New()
	rep := &reports{}
	b.Journal(rep)
	b.SetDaemonOwner("owner@h")
	next := b.nextRecordID
	admins := b.records[AdministratorsGroup]
	admins.Allow = []string{"legacy@h", "owner@h"}
	b.Restore(ports.Snapshot{Records: []protocol.Record{admins}})
	if b.IsAdministrator("legacy@h") || b.IsPerson("legacy@h") {
		t.Fatal("an administrator with no User was repaired into one")
	}
	if !rep.has("stored @administrators member legacy@h is ignored") {
		t.Errorf("the ignored administrator was not reported: %v", rep.lines)
	}
	if b.nextRecordID != next {
		t.Errorf("the restore took record IDs: %d, then %d", next, b.nextRecordID)
	}
	if !b.IsAdministrator("owner@h") {
		t.Error("the daemon Owner stopped being an administrator")
	}
}
