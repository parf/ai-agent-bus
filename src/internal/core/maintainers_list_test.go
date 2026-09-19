package core

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func maintainersFixture(t *testing.T) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("admin@h")
	for _, name := range []string{"owner@h", "direct@h", "nested@h", "outsider@h", "operator@h"} {
		if _, err := b.SetUser("admin@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.SetGroup("admin@h", AdministratorsGroup, []string{"admin@h", "operator@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", "@inner", []string{"nested@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", "@outer", []string{"@inner"}); err != nil {
		t.Fatal(err)
	}
	for _, record := range []protocol.Record{
		{Name: "service@h", Owner: "owner@h", Kind: protocol.KindAgent},
		{Name: "agent@h", Owner: "owner@h", Kind: "agent"},
		{Name: "topic@h", Owner: "owner@h", Kind: protocol.KindQueue},
		{Name: "target@h", Owner: "owner@h", Kind: protocol.KindAgent},
	} {
		if _, err := b.Register(record); err != nil {
			t.Fatal(err)
		}
	}
	return b
}

func TestMaintainersListGrantsDirectAndNestedManagement(t *testing.T) {
	b := maintainersFixture(t)
	terms := protocol.MaintainerList{"direct@h", "service@h", "agent@h", "@outer", AdministratorsGroup}
	got, err := b.Manage("owner@h", Management{Name: "target@h", Maintainers: &terms})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Maintainers, terms) {
		t.Fatalf("maintainers changed on write: %#v", got.Maintainers)
	}
	for _, caller := range []string{"target@h", "direct@h", "service@h", "agent@h", "nested@h", "operator@h"} {
		descr := "managed by " + caller
		if _, err := b.Manage(caller, Management{Name: "target@h", Descr: &descr}); err != nil {
			t.Errorf("%s did not gain management: %v", caller, err)
		}
	}
	if _, err := b.Manage("outsider@h", Management{Name: "target@h", Descr: ptr("refused")}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("outsider gained management: %v", err)
	}
	if _, err := b.Manage("direct@h", Management{Name: "target@h", Maintainers: &protocol.MaintainerList{"outsider@h"}}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("Maintainer replaced Maintainers: %v", err)
	}
	replacement := protocol.MaintainerList{"outsider@h"}
	if _, err := b.Manage("admin@h", Management{Name: "target@h", Maintainers: &replacement}); err != nil {
		t.Fatalf("daemon Owner could not replace Maintainers: %v", err)
	}
}

func TestMaintainersGroupMembershipRevokesOnNextOperation(t *testing.T) {
	b := maintainersFixture(t)
	terms := protocol.MaintainerList{"@outer"}
	if _, err := b.Manage("owner@h", Management{Name: "target@h", Maintainers: &terms}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("nested@h", Management{Name: "target@h", Descr: ptr("before")}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("admin@h", "@inner", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("nested@h", Management{Name: "target@h", Descr: ptr("after")}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("removed nested member retained management: %v", err)
	}
}

func TestMaintainersReplacementIsAtomicAndKindChecked(t *testing.T) {
	b := maintainersFixture(t)
	original := protocol.MaintainerList{"direct@h"}
	if _, err := b.Manage("owner@h", Management{Name: "target@h", Maintainers: &original}); err != nil {
		t.Fatal(err)
	}
	for name, terms := range map[string]protocol.MaintainerList{
		"wildcard":  {"*"},
		"unknown":   {"outsider@h", "missing@h"},
		"topic":     {"topic@h"},
		"duplicate": {"direct@h", " DIRECT@H "},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := b.Manage("owner@h", Management{Name: "target@h", Maintainers: &terms}); err == nil {
				t.Fatalf("invalid Maintainers list was accepted: %#v", terms)
			}
			got, _ := b.Lookup("owner@h", "target@h")
			if !reflect.DeepEqual(got.Maintainers, original) {
				t.Fatalf("failed replacement partly applied: %#v", got.Maintainers)
			}
		})
	}
	empty := protocol.MaintainerList{}
	if _, err := b.Manage("owner@h", Management{Name: "target@h", Maintainers: &empty}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("direct@h", Management{Name: "target@h", Descr: ptr("after clear")}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("cleared direct Maintainer retained management: %v", err)
	}
}

func TestMaintainersRefreshAndSnapshotUseArrayWithoutAuthorityLoss(t *testing.T) {
	b := maintainersFixture(t)
	terms := protocol.MaintainerList{"direct@h", "@outer"}
	if _, err := b.Manage("owner@h", Management{Name: "target@h", Maintainers: &terms}); err != nil {
		t.Fatal(err)
	}
	// Registration never writes Maintainers. A service refresh preserves the
	// owner's list even when a caller supplies a contrary value.
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "target@h", Owner: "target@h", Maintainers: protocol.MaintainerList{"outsider@h"}}); err != nil {
		t.Fatal(err)
	}
	got, _ := b.Lookup("owner@h", "target@h")
	if !reflect.DeepEqual(got.Maintainers, terms) {
		t.Fatalf("refresh changed Maintainers: %#v", got.Maintainers)
	}
	encoded, err := json.Marshal(b.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"maintainers":["direct@h","@outer"]`) || strings.Contains(string(encoded), `"maintainers":"`) {
		t.Fatalf("snapshot did not self-complete to array spelling: %s", encoded)
	}
	var snapshot ports.Snapshot
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		t.Fatal(err)
	}
	restarted := New()
	restarted.Restore(snapshot)
	if _, err := restarted.Manage("direct@h", Management{Name: "target@h", Descr: ptr("restored")}); err != nil {
		t.Fatalf("direct Maintainer lost authority after restore: %v", err)
	}
}
