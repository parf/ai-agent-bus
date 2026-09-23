package core

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// An inactive record is no such entity to everything but its reactivation and
// the read-only view (docs/constitution.md#common-record-fields).
func statusFixture(t *testing.T) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "maint@h", "reader@h", "stranger@h", "#caller@h")
	provision(t, b, nil, protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h",
		Allow: []string{"reader@h", "#caller@h"}, Maintainers: protocol.MaintainerList{"maint@h"}, Full: protocol.OverflowStrict})
	if _, err := b.Send(protocol.Envelope{From: "#caller@h", To: "jobs@h", Body: "kept"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Status: ptr(protocol.StatusInactive)}); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAnInactiveRecordIsNoSuchEntity(t *testing.T) {
	b := statusFixture(t)
	for _, who := range []string{"alice@h", "reader@h", "admin@h"} {
		if _, ok := b.Lookup(who, "jobs@h"); ok {
			t.Errorf("%s looked up an inactive record", who)
		}
		for _, r := range b.List(who, "") {
			if r.Name == "jobs@h" {
				t.Errorf("%s listed an inactive record", who)
			}
		}
	}
	if _, err := b.Send(protocol.Envelope{From: "#caller@h", To: "jobs@h", Body: "x"}); !errors.Is(err, ErrUnknown) {
		t.Errorf("a send to an inactive record: %v", err)
	}
	if err := b.Unregister("jobs@h", "alice@h"); !errors.Is(err, ErrUnknown) {
		t.Errorf("an inactive record was removed: %v", err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Descr: ptr("x")}); !errors.Is(err, ErrUnknown) {
		t.Errorf("an inactive record took an edit other than its reactivation: %v", err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Status: ptr(protocol.StatusActive), Descr: ptr("x")}); !errors.Is(err, ErrUnknown) {
		t.Errorf("a reactivation carried another field: %v", err)
	}
	// Its name stays reserved.
	if _, err := b.Register(protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h"}); !errors.Is(err, ErrExists) {
		t.Errorf("a registration took an inactive record's name: %v", err)
	}
	if _, err := b.Register(protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "stranger@h"}); !errors.Is(err, ErrExists) {
		t.Errorf("a stranger registered over an inactive record's name: %v", err)
	}
}

// Only the Owner or a Maintainer reactivates, by a status edit, and what was
// queued is still there.
func TestOnlyTheOwnerOrAMaintainerReactivates(t *testing.T) {
	b := statusFixture(t)
	for _, who := range []string{"reader@h", "stranger@h"} {
		// No such record to anyone who could not reactivate it: not even its
		// status is disclosed.
		if _, err := b.Manage(who, Management{Name: "jobs@h", Status: ptr(protocol.StatusActive)}); !errors.Is(err, ErrUnknown) {
			t.Errorf("%s reactivating a record it does not manage: %v, want unknown", who, err)
		}
	}
	if _, err := b.Manage("maint@h", Management{Name: "jobs@h", Status: ptr(protocol.StatusActive)}); err != nil {
		t.Fatalf("a Maintainer could not reactivate: %v", err)
	}
	r, ok := b.Lookup("reader@h", "jobs@h")
	if !ok || r.Queued != 1 || r.Status != protocol.StatusActive {
		t.Fatalf("the reactivated record is %+v, %v; want it back with its work", r, ok)
	}
}

// The read-only view shows an inactive record to the actors its ACL admits
// and to the daemon Owner, and to nobody else.
func TestTheInactiveViewShowsWhomTheACLAdmitsAndTheOwner(t *testing.T) {
	b := statusFixture(t)
	has := func(who string) bool {
		for _, r := range b.Inactive(who) {
			if r.Name == "jobs@h" && r.Status == protocol.StatusInactive {
				return true
			}
		}
		return false
	}
	for who, want := range map[string]bool{"alice@h": true, "maint@h": true, "reader@h": true, "admin@h": true, "stranger@h": false} {
		if got := has(who); got != want {
			t.Errorf("%s sees the inactive record: %v, want %v", who, got, want)
		}
	}
	// And an active record is not in it.
	for _, r := range b.Inactive("admin@h") {
		if r.Name == "alice@h" {
			t.Error("an active user's record is in the inactive view")
		}
	}
}

// An inactive caller is refused as suspended; an inactive target is unknown.
func TestAnInactiveCallerIsSuspendedAndAnInactiveTargetUnknown(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h")
	provision(t, b, nil, protocol.Record{Name: "#worker@h", Kind: protocol.KindAgent, Owner: "alice@h", Allow: []string{"*"}, Full: protocol.OverflowStrict})
	if _, err := b.SetUserState("admin@h", "alice@h", protocol.StatusInactive); err != nil {
		t.Fatal(err)
	}
	if err := b.Authenticate("alice@h"); !errors.Is(err, ErrInactive) {
		t.Errorf("an inactive user authenticated: %v", err)
	}
	if err := b.Authenticate("#worker@h"); !errors.Is(err, ErrInactive) {
		t.Errorf("an inactive user's agent authenticated: %v", err)
	}
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "#worker@h"}); !errors.Is(err, ErrUnknown) {
		t.Errorf("a send to an inactive user's agent: %v", err)
	}
	// A User's own record takes its status from the User.
	if _, err := b.Manage("admin@h", Management{Name: "bob@h", Status: ptr(protocol.StatusInactive)}); !errors.Is(err, ErrBadName) {
		t.Errorf("a user record took a status edit of its own: %v", err)
	}
	// The daemon Owner stays active.
	if _, err := b.SetUserState("admin@h", "admin@h", protocol.StatusInactive); !errors.Is(err, ErrNotOwner) {
		t.Errorf("the daemon owner was deactivated: %v", err)
	}
}

// An Administrator changes ordinary Users, never another Administrator; the
// daemon Owner changes either.
func TestWhoChangesAUsersStatus(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "ops@h", "peer@h", "plain@h")
	if err := b.SetGroup("admin@h", AdministratorsGroup, []string{"admin@h", "ops@h", "peer@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetUserState("ops@h", "plain@h", protocol.StatusInactive); err != nil {
		t.Fatalf("an Administrator could not deactivate an ordinary user: %v", err)
	}
	if _, err := b.SetUserState("ops@h", "plain@h", protocol.StatusActive); err != nil {
		t.Fatalf("an Administrator could not reactivate an ordinary user: %v", err)
	}
	if _, err := b.SetUserState("ops@h", "peer@h", protocol.StatusInactive); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("an Administrator changed another Administrator: %v", err)
	}
	if _, err := b.SetUserState("admin@h", "peer@h", protocol.StatusInactive); err != nil {
		t.Fatalf("the daemon Owner could not deactivate an Administrator: %v", err)
	}
	if _, err := b.SetUserState("admin@h", "peer@h", "banned"); !errors.Is(err, ErrProfile) {
		t.Fatalf("a banned state was accepted: %v", err)
	}
}

// A restored inactive record stays inactive: hidden, reserved, and in the view.
func TestARestoredInactiveRecordStaysInactive(t *testing.T) {
	b := New()
	b.Restore(withUsers(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "owner@h", Status: protocol.StatusInactive, Full: protocol.OverflowStrict},
	}}, "owner@h"))
	if err := b.EstablishDaemonOwner("owner@h"); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.Lookup("owner@h", "jobs@h"); ok {
		t.Error("a restored inactive record is visible")
	}
	if got := b.Inactive("owner@h"); len(got) != 1 || got[0].Name != "jobs@h" {
		t.Errorf("the restored inactive record is not in the view: %+v", got)
	}
	// A status that is neither is a record no write produced.
	rep := &reports{}
	c := New()
	c.Journal(rep)
	c.Restore(withUsers(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "odd@h", Kind: protocol.KindQueue, Owner: "owner@h", Status: "paused", Full: protocol.OverflowStrict},
	}}, "owner@h"))
	if !rep.has("stored record odd@h is ignored") {
		t.Errorf("a record with an unknown status was loaded: %v", rep.lines)
	}
}
