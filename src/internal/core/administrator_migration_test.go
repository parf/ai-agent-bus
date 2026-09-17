package core

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestAdministratorMigrationPreservesGrantsWithoutPromotion(t *testing.T) {
	s := ports.Snapshot{
		Users: []protocol.User{{Name: "owner@h", State: "active"}, {Name: "ordinary@h", State: "active"}},
		Groups: map[string][]string{
			"@maintainers":           {"owner@h", "admin@h"},
			"@administrators":        {"ordinary@h"},
			"@administrators-legacy": {"other@h"},
		},
		Records: []protocol.Record{
			{Name: "admin-managed@h", Owner: "owner@h", Maintainers: "@maintainers", Allow: []string{"owner@h"}},
			{Name: "ordinary-managed@h", Owner: "owner@h", Maintainers: "@administrators", Allow: []string{"owner@h"}},
			{Name: "admin-visible@h", Owner: "owner@h", Allow: []string{"@maintainers"}},
			{Name: "ordinary-visible@h", Owner: "owner@h", Allow: []string{"@administrators"}},
			{Name: "unresolved@h", Owner: "owner@h", Allow: []string{"@administrators-legacy-1"}},
		},
		Queues: []ports.Queue{{Name: "ordinary-managed@h", In: 1, Messages: []protocol.Envelope{{Body: "kept"}}}},
	}
	original, _ := json.Marshal(s)
	b := New()
	b.Restore(s)
	b.SetDaemonOwner("owner@h")
	if !b.IsAdministrator("admin@h") || !b.IsPerson("admin@h") {
		t.Fatal("legacy administrator lost administrative standing or profile")
	}
	if b.IsAdministrator("ordinary@h") {
		t.Fatal("ordinary colliding-group member acquired administration")
	}
	if _, err := b.SetUser("ordinary@h", protocol.User{Name: "new@h"}, true); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("ordinary member can administer users: %v", err)
	}
	for _, check := range []struct {
		who, record string
		allowed     bool
	}{
		{"admin@h", "admin-managed@h", true}, {"ordinary@h", "ordinary-managed@h", true},
		{"ordinary@h", "admin-managed@h", false}, {"admin@h", "ordinary-managed@h", false},
	} {
		_, err := b.Manage(check.who, Management{Name: check.record, Descr: ptr("edited")})
		if (err == nil) != check.allowed {
			t.Fatalf("management %s -> %s: %v, allowed=%v", check.who, check.record, err, check.allowed)
		}
	}
	for _, check := range []struct {
		who, record string
		allowed     bool
	}{
		{"admin@h", "admin-visible@h", true}, {"ordinary@h", "ordinary-visible@h", true},
		{"ordinary@h", "admin-visible@h", false}, {"admin@h", "ordinary-visible@h", false},
		{"ordinary@h", "unresolved@h", false},
	} {
		if _, visible := b.Lookup(check.who, check.record); visible != check.allowed {
			t.Fatalf("visibility %s -> %s: %v, want %v", check.who, check.record, visible, check.allowed)
		}
	}
	groups := b.Groups("owner@h")
	if _, exists := groups["@maintainers"]; exists || !reflect.DeepEqual(groups["@administrators-legacy-2"], []string{"ordinary@h"}) {
		t.Fatalf("wrong migrated groups: %v", groups)
	}
	after, _ := json.Marshal(s)
	if string(original) != string(after) {
		t.Fatal("restore modified the source snapshot")
	}
	saved := b.Snapshot()
	if !reflect.DeepEqual(saved.Queues, s.Queues) {
		t.Fatal("migration changed queued work")
	}
	// A real serialization/restart must neither re-rename groups nor re-grant roles.
	encoded, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	var disk ports.Snapshot
	if err := json.Unmarshal(encoded, &disk); err != nil {
		t.Fatal(err)
	}
	restarted := New()
	restarted.Restore(disk)
	restarted.SetDaemonOwner("owner@h")
	if !reflect.DeepEqual(groups, restarted.Groups("owner@h")) || restarted.IsAdministrator("ordinary@h") || !restarted.IsAdministrator("admin@h") {
		t.Fatal("a second restore changed groups or administrative standing")
	}
	for _, record := range saved.Records {
		got, ok := restarted.Lookup("owner@h", record.Name)
		if !ok || got.Maintainers != record.Maintainers || !reflect.DeepEqual(got.Allow, record.Allow) {
			t.Fatalf("second restore changed record grants for %s", record.Name)
		}
	}
}

func TestRetiredAdministrativeNameCannotBeRecreated(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")
	for _, who := range []string{"owner@h", "admin@h"} {
		if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h", "admin@h"}); err != nil {
			t.Fatal(err)
		}
		if err := b.SetGroup(who, "@maintainers", []string{"ordinary@h"}); !errors.Is(err, ErrBadName) {
			t.Fatalf("%s recreated the migration trigger: %v", who, err)
		}
	}
	if err := b.SetGroup("admin@h", "@ops", []string{"ordinary@h"}); err != nil {
		t.Fatal(err)
	}
	if b.IsAdministrator("ordinary@h") {
		t.Fatal("ordinary group granted administration")
	}
}

func TestAdministratorMigrationWithoutCollision(t *testing.T) {
	b := New()
	b.Restore(ports.Snapshot{Groups: map[string][]string{"@maintainers": {"owner@h", "admin@h"}}})
	b.SetDaemonOwner("owner@h")
	groups := b.Groups("owner@h")
	if len(groups) != 1 || !reflect.DeepEqual(groups[AdministratorsGroup], []string{"owner@h", "admin@h"}) || !b.IsAdministrator("admin@h") {
		t.Fatalf("ordinary upgrade did not preserve its sole administrative group: %v", groups)
	}
}
