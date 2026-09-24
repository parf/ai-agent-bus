package core

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The Users listing reads memberships and ownership from one index built per
// answer instead of rescanning every record for every row. Each row must be
// exactly what the single-user view says: nested groups, the protected
// Administrator group as a boundary, a cycle, a group and records made inert
// by an inactive owner, and records owned by more than one User.
func TestUsersListingMatchesTheSingleUserViewForEveryRow(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")
	people := []string{"admin@h", "alice@h", "bob@h", "carol@h", "dave@h", "gone@h"}
	for _, name := range people {
		if _, err := b.SetUser("owner@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h", "admin@h"}))
	// @outer reaches carol through @inner, and @inner reaches back: a cycle.
	must(b.SetGroup("owner@h", "@outer", []string{"alice@h"}))
	must(b.SetGroup("owner@h", "@inner", []string{"carol@h", "@outer"}))
	must(b.SetGroup("owner@h", "@outer", []string{"alice@h", "@inner"}))
	// Naming the Administrator group grants through it but does not expand
	// past it: bob is not reached through @admins-too.
	must(b.SetGroup("owner@h", "@admins-too", []string{AdministratorsGroup, "dave@h"}))
	// A group whose owner goes inactive stops counting.
	must(b.SetGroup("gone@h", "@gone-team", []string{"bob@h", "alice@h"}))
	for i, owner := range []string{"alice@h", "alice@h", "bob@h", "gone@h", "carol@h"} {
		if _, err := b.Register(protocol.Record{Kind: protocol.KindQueue, Name: fmt.Sprintf("q%d@h", i), Owner: owner}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.SetUserState("owner@h", "gone@h", protocol.StatusInactive); err != nil {
		t.Fatal(err)
	}
	for _, caller := range []string{"owner@h", "admin@h", "alice@h"} {
		rows := b.Users(caller, []string{"#stray@h"})
		if caller != "alice@h" && len(rows) < len(people) {
			t.Fatalf("%s sees %d rows", caller, len(rows))
		}
		grouped, owned := 0, 0
		b.mu.Lock()
		for _, row := range rows {
			want := b.userView(caller, row.Name)
			if !reflect.DeepEqual(row, want) {
				t.Errorf("%s: listing row %s = %+v, single view %+v", caller, row.Name, row, want)
			}
			grouped += len(row.Groups)
			owned += len(row.Services)
		}
		b.mu.Unlock()
		// Positive control: the rows compared carry memberships and records,
		// so two empty answers cannot agree by saying nothing.
		if caller == "owner@h" && (grouped < 6 || owned < 4) {
			t.Errorf("fixture reached %d memberships and %d owned records", grouped, owned)
		}
	}
}
