package core

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A Group is an ordinary record whose allow is its membership
// (docs/constitution.md#-group).
func TestAGroupIsARecordInTheSharedIDSpace(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h")
	if err := b.SetGroup("alice@h", "@crew", []string{"bob@h", "alice@h"}); err != nil {
		t.Fatalf("a User could not create a group: %v", err)
	}
	g := b.records["@crew"]
	if g.Kind != protocol.KindGroup || g.Owner != "alice@h" || len(g.Allow) != 2 || g.ID == 0 {
		t.Fatalf("the group record is %+v", g)
	}
	for name, r := range b.records {
		if name != "@crew" && r.ID == g.ID {
			t.Fatalf("the group's ID %d is also %s's", g.ID, name)
		}
	}
	if !b.member("bob@h", "@crew") {
		t.Fatal("the group's allow list is not its membership")
	}
	// The name says the kind, both ways.
	if _, err := b.Register(protocol.Record{Name: "crew@h", Kind: protocol.KindGroup, Owner: "alice@h"}); !errors.Is(err, ErrKind) {
		t.Errorf("a group registered without its @: %v", err)
	}
	if _, err := b.Register(protocol.Record{Name: "@jobs", Kind: protocol.KindQueue, Owner: "alice@h"}); !errors.Is(err, ErrKind) {
		t.Errorf("a queue registered under a group's name: %v", err)
	}
	// Retired by emptying, never removed.
	if err := b.Unregister("@crew", "alice@h"); !errors.Is(err, ErrNoRemoval) {
		t.Errorf("a group was removed: %v", err)
	}
	// A restored group record without its kind is not a group.
	rep := &reports{}
	c := New()
	c.Journal(rep)
	c.Restore(withUsers(ports.Snapshot{Clean: true, Records: []protocol.Record{{Name: "@loose", Owner: "owner@h", Allow: []string{"owner@h"}}}}, "owner@h"))
	if !rep.has("stored record @loose is ignored") || c.member("owner@h", "@loose") {
		t.Errorf("a group record without its kind was loaded: %v", rep.lines)
	}
}

// Its Owner, its Maintainers and the daemon's Administrators change its
// membership; nobody else does.
func TestWhoChangesAGroupsMembership(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "maint@h", "stranger@h", "ops@h")
	if err := b.SetGroup("admin@h", AdministratorsGroup, []string{"admin@h", "ops@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("alice@h", "@crew", []string{"alice@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "@crew", Maintainers: ptr(protocol.MaintainerList{"maint@h"})}); err != nil {
		t.Fatal(err)
	}
	for who, want := range map[string]bool{"alice@h": true, "maint@h": true, "ops@h": true, "stranger@h": false} {
		err := b.SetGroup(who, "@crew", []string{"alice@h", who})
		if (err == nil) != want {
			t.Errorf("%s changing the membership: %v, want allowed %v", who, err, want)
		}
	}
	// Membership is actors: never the wildcard or a runtime term.
	for _, bad := range []string{"*", "@owner", "@agent"} {
		if err := b.SetGroup("alice@h", "@crew", []string{bad}); !errors.Is(err, ErrBadName) {
			t.Errorf("%s became a member: %v", bad, err)
		}
		if _, err := b.Manage("alice@h", Management{Name: "@crew", Allow: &[]string{bad}}); !errors.Is(err, ErrBadName) {
			t.Errorf("%s became a member through its allow list: %v", bad, err)
		}
	}
}

// The protected group follows daemon ownership, has no Maintainers, and only
// the daemon Owner changes its membership; a Group Maintainer cannot grant
// daemon administration through it.
func TestTheAdministratorsGroupFollowsTheDaemonOwner(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "next@h", "ops@h", "maint@h")
	if got := b.records[AdministratorsGroup]; got.Owner != "admin@h" || got.Kind != protocol.KindGroup {
		t.Fatalf("the administrators group is %+v", got)
	}
	for what, change := range map[string]Management{
		"owner":       {Name: AdministratorsGroup, Owner: ptr("next@h")},
		"maintainers": {Name: AdministratorsGroup, Maintainers: ptr(protocol.MaintainerList{"maint@h"})},
		"members":     {Name: AdministratorsGroup, Allow: &[]string{"admin@h", "maint@h"}},
	} {
		if _, err := b.Manage("admin@h", change); !errors.Is(err, ErrNotOwner) {
			t.Errorf("the administrators group took a %s change through management: %v", what, err)
		}
	}
	// A Group Maintainer is not the daemon Owner: it cannot add to the
	// protected group, directly or by nesting its own group in it.
	if err := b.SetGroup("admin@h", "@ops", []string{"ops@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("admin@h", Management{Name: "@ops", Maintainers: ptr(protocol.MaintainerList{"maint@h"})}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("maint@h", AdministratorsGroup, []string{"admin@h", "maint@h"}); !errors.Is(err, ErrNotOwner) {
		t.Errorf("a group maintainer changed the administrators: %v", err)
	}
	if err := b.SetGroup("admin@h", AdministratorsGroup, []string{"admin@h", "@ops"}); !errors.Is(err, ErrBadName) {
		t.Errorf("a group was nested in the administrators: %v", err)
	}
	if b.IsAdministrator("ops@h") || b.IsAdministrator("maint@h") {
		t.Fatal("daemon administration was granted through an ordinary group")
	}
	// Daemon ownership moves the group's Owner in the same write.
	if _, err := b.TransferDaemonOwner("admin@h", "next@h"); err != nil {
		t.Fatal(err)
	}
	if got := b.records[AdministratorsGroup]; got.Owner != "next@h" {
		t.Fatalf("the administrators group stayed %s's after the daemon owner changed", got.Owner)
	}
	// A restore whose administrators group is not the daemon Owner's refuses.
	snap := b.Snapshot()
	for i, r := range snap.Records {
		if r.Name == AdministratorsGroup {
			snap.Records[i].Owner = "admin@h"
		}
	}
	c := New()
	c.Restore(snap)
	if err := c.EstablishDaemonOwner("next@h"); err == nil {
		t.Fatal("a start accepted an administrators group owned by someone other than the daemon owner")
	}
}

// An inactive Group grants nothing: no membership path through it, and at a
// publication it is ignored with a log line and counted as no drop.
func TestAnInactiveGroupGrantsNothing(t *testing.T) {
	b := New()
	rep := &reports{}
	b.Journal(rep)
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h")
	provision(t, b, nil,
		protocol.Record{Name: "#box@h", Kind: protocol.KindAgent, Owner: "bob@h", Full: protocol.OverflowStrict},
		protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h", Allow: []string{"@crew"}, Full: protocol.OverflowStrict},
		protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}},
	)
	if err := b.SetGroup("alice@h", "@crew", []string{"bob@h", "#box@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "news@h", Subs: &[]string{"@crew"}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.Lookup("bob@h", "jobs@h"); !ok {
		t.Fatal("control: the group did not grant access")
	}
	if _, err := b.Manage("alice@h", Management{Name: "@crew", Status: ptr(protocol.StatusInactive)}); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.Lookup("bob@h", "jobs@h"); ok {
		t.Error("an inactive group still grants access")
	}
	if b.member("bob@h", "@crew") {
		t.Error("an inactive group still has members")
	}
	if _, err := b.Send(protocol.Envelope{From: "alice@h", To: "news@h", Body: "x"}); err == nil {
		t.Error("a publication whose only recipient was an inactive group was accepted")
	}
	if !rep.has("ignored its deliver-to group @crew, which is inactive") {
		t.Errorf("the inactive group term was not logged: %v", rep.lines)
	}
	if in := b.inboxes["#box@h"]; in != nil && in.dropped != 0 {
		t.Error("an inactive group counted a drop against its member")
	}
}

// Every member of a Group reads its private values: it has no principal of
// its own (docs/constitution.md#-private-values).
func TestAGroupsSecretIsReadByItsMembers(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h", "stranger@h")
	if err := b.SetGroup("alice@h", "@crew", []string{"bob@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetSecret("@crew", "alice@h", "TOKEN=shared"); err != nil {
		t.Fatal(err)
	}
	if got, err := b.Secret("@crew", "bob@h"); err != nil || got != "TOKEN=shared" {
		t.Fatalf("a member read %q, %v", got, err)
	}
	if _, err := b.Secret("@crew", "stranger@h"); !errors.Is(err, ErrUnknown) {
		t.Fatalf("a stranger read the group's secret: %v", err)
	}
}
