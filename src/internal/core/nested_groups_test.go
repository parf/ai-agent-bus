package core

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func nestedGroupsFixture(t *testing.T) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("owner@h")
	for _, name := range []string{"admin@h", "alice@h", "reader@h", "future@h", "stranger@h"} {
		if _, err := b.SetUser("owner@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.Register(protocol.Record{Name: "worker@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h", "admin@h"}); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestNestedGroupsGrantAccessAndManagement(t *testing.T) {
	b := nestedGroupsFixture(t)
	if err := b.SetGroup("admin@h", "@leaf", []string{"reader@h", "worker@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", "@middle", []string{"@leaf"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", "@outer", []string{"@middle", "@missing"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "shared@h", Owner: "alice@h", Allow: []string{"@outer"}, NoMaster: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "managed@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "managed@h", Maintainers: ptr(protocol.MaintainerList{"@outer"})}); err != nil {
		t.Fatal(err)
	}
	for _, caller := range []string{"reader@h", "worker@h"} {
		if _, err := b.Send(protocol.Envelope{From: caller, To: "shared@h", Body: caller}); err != nil {
			t.Fatalf("nested ACL refused %s: %v", caller, err)
		}
	}
	if _, err := b.Send(protocol.Envelope{From: "stranger@h", To: "shared@h"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("stranger received nested grant: %v", err)
	}
	for _, caller := range []string{"reader@h", "worker@h"} {
		descr := "changed through nested Maintainers by " + caller
		if _, err := b.Manage(caller, Management{Name: "managed@h", Descr: &descr}); err != nil {
			t.Fatalf("nested Maintainer %s could not manage: %v", caller, err)
		}
	}
	if got, _ := b.Lookup("reader@h", "managed@h"); got.Descr != "changed through nested Maintainers by worker@h" {
		t.Fatalf("nested Maintainer change missing: %+v", got)
	}

	var reader protocol.User
	for _, user := range b.Users("admin@h", nil) {
		if user.Name == "reader@h" {
			reader = user
		}
	}
	if !reflect.DeepEqual(reader.Groups, []string{"@leaf", "@middle", "@outer"}) {
		t.Fatalf("effective memberships not published: %v", reader.Groups)
	}
}

func TestNestedGroupsResolveUnknownsCyclesAndRevocation(t *testing.T) {
	b := nestedGroupsFixture(t)
	if err := b.SetGroup("admin@h", "@future-edge", []string{"@future"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "future-box@h", Owner: "alice@h", Allow: []string{"@future-edge"}, NoMaster: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "future@h", To: "future-box@h"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("unknown subgroup was not inert: %v", err)
	}
	if err := b.SetGroup("admin@h", "@future", []string{"future@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "future@h", To: "future-box@h"}); err != nil {
		t.Fatalf("populated subgroup did not become effective: %v", err)
	}

	if err := b.SetGroup("admin@h", "@cycle-a", []string{"@cycle-a", "@cycle-b"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", "@cycle-b", []string{"@cycle-a", "reader@h"}); err != nil {
		t.Fatal(err)
	}
	if !b.member("reader@h", "@cycle-a") || b.member("stranger@h", "@cycle-a") {
		t.Fatal("cycle did not terminate with graph-reachability semantics")
	}

	if err := b.SetGroup("admin@h", "@nested-reader", []string{"@reader-leaf"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", "@reader-leaf", []string{"reader@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "wait@h", Owner: "alice@h", Allow: []string{"@nested-reader"}, NoMaster: true}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stopped := make(chan error, 1)
	go func() {
		_, err := b.ConsumeAs(ctx, "reader@h", "wait@h", "", "", false, false)
		stopped <- err
	}()
	waitForWaiters(t, b, "wait@h", 1)
	if err := b.SetGroup("admin@h", "@reader-leaf", nil); err != nil {
		t.Fatal(err)
	}
	if err := <-stopped; !errors.Is(err, ErrNotAllow) {
		t.Fatalf("nested membership removal ended read with %v", err)
	}
}

func TestAdministratorMembershipStaysDirect(t *testing.T) {
	b := nestedGroupsFixture(t)
	if err := b.SetGroup("admin@h", "@ops", []string{"reader@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h", "admin@h", "@ops"}); !errors.Is(err, ErrBadName) {
		t.Fatalf("protected group accepted nested membership: %v", err)
	}
	if b.IsAdministrator("reader@h") {
		t.Fatal("ordinary subgroup promoted its member to Administrator")
	}
	if !b.IsAdministrator("owner@h") || !b.IsAdministrator("admin@h") {
		t.Fatal("refused protected write changed direct Administrators")
	}

	// Naming @administrators inside an ordinary group is an access edge into
	// its direct membership, never a way to nest authority into the protected
	// group itself.
	if err := b.SetGroup("admin@h", "@admin-readers", []string{AdministratorsGroup}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "admin-box@h", Owner: "alice@h", Allow: []string{"@admin-readers"}, NoMaster: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "admin@h", To: "admin-box@h"}); err != nil {
		t.Fatalf("ordinary group did not resolve direct Administrators for access: %v", err)
	}
	if _, err := b.Send(protocol.Envelope{From: "reader@h", To: "admin-box@h"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("non-Administrator followed protected access edge: %v", err)
	}
}

func TestNestedGroupsPersistAndDamagedProtectedNestingFailsStartup(t *testing.T) {
	b := nestedGroupsFixture(t)
	if err := b.SetGroup("admin@h", "@leaf", []string{"reader@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", "@outer", []string{"@leaf"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "persisted@h", Owner: "alice@h", Allow: []string{"@outer"}, NoMaster: true}); err != nil {
		t.Fatal(err)
	}

	snapshot := b.Snapshot()
	recovered := New()
	recovered.Restore(snapshot)
	if err := recovered.EstablishDaemonOwner("contrary@h"); err != nil {
		t.Fatal(err)
	}
	if _, err := recovered.Send(protocol.Envelope{From: "reader@h", To: "persisted@h"}); err != nil {
		t.Fatalf("nested grant did not survive restore: %v", err)
	}

	damaged := snapshot
	damaged.Groups = make(map[string][]string, len(snapshot.Groups))
	for name, members := range snapshot.Groups {
		damaged.Groups[name] = append([]string(nil), members...)
	}
	damaged.Groups["@ops"] = []string{"reader@h"}
	damaged.Groups[AdministratorsGroup] = append(damaged.Groups[AdministratorsGroup], "@ops")
	broken := New()
	broken.Restore(damaged)
	if err := broken.EstablishDaemonOwner("owner@h"); err == nil {
		t.Fatal("startup accepted nested protected-group membership")
	}
	if broken.IsAdministrator("reader@h") {
		t.Fatal("damaged protected nesting promoted an ordinary member before refusal")
	}
	if _, exists := broken.users["@ops"]; exists {
		t.Fatal("damaged protected nesting manufactured a group-shaped user")
	}
}
