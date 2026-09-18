package main

import (
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The directory answers "who can use this node" first. A state column made
// every row carry the word active, which is the ordinary majority and so
// marked nothing; the state now sits beside the name it belongs to, on the
// minority of rows that are not active, and the list opens on active users.
func TestTheUserDirectoryOpensOnActiveUsersAndMarksOnlyTheOthers(t *testing.T) {
	m := meaningFixture(t)
	for _, who := range []string{"steady@h", "resting@h", "barred@h"} {
		if _, err := m.bus.SetUser("admin@h", protocol.User{Name: who, PersonName: "Person " + who}, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.bus.SetUserState("admin@h", "resting@h", "paused"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.bus.SetUserState("admin@h", "barred@h", "banned"); err != nil {
		t.Fatal(err)
	}

	// Default view: active only. Three states exist in the fixture, so a
	// filter that did nothing would show all three here.
	first := m.get("/users")
	if !strings.Contains(first, "steady@h") {
		t.Fatalf("the default directory does not list an active user: %s", first)
	}
	for _, hidden := range []string{"resting@h", "barred@h"} {
		if strings.Contains(first, ">"+hidden+"<") {
			t.Errorf("the default directory lists %s, which is not active", hidden)
		}
	}
	// The column is gone; nothing says "active" on a row that is.
	if strings.Contains(first, "<th scope=col>State</th>") {
		t.Errorf("the directory still carries a State column: %s", first)
	}
	// Element form: the class names live in the inline stylesheet that every
	// page carries, so the bare string matches everywhere.
	if strings.Contains(first, `<span class="state-badge`) {
		t.Error("an active row carries a state marker")
	}

	// Each state is reachable and counted, and each marks its rows.
	for _, probe := range []struct {
		query, name, badge string
	}{
		{"?state=paused", "resting@h", `<span class="state-badge state-inactive">INACTIVE</span>`},
		{"?state=banned", "barred@h", `<span class="state-badge state-banned">BANNED</span>`},
	} {
		body := m.get("/users" + probe.query)
		if !strings.Contains(body, probe.badge) {
			t.Errorf("%s does not mark its rows: %s", probe.query, body)
		}
		if !strings.Contains(body, "<s>"+probe.name+"</s>") {
			t.Errorf("%s does not strike the name of %s: %s", probe.query, probe.name, body)
		}
		if strings.Contains(body, ">steady@h<") {
			t.Errorf("%s also lists the active user", probe.query)
		}
	}

	// Every state at once remains reachable, or the directory would have no
	// answer to "show me everyone".
	all := m.get("/users?state=all")
	for _, who := range []string{"steady@h", "resting@h", "barred@h"} {
		if !strings.Contains(all, who) {
			t.Errorf("the all-states view is missing %s", who)
		}
	}

	// The counts are of users in each state, not of the rows on the page.
	states := section(t, first, `<nav class=filter-nav aria-label="User state filter">`, "</nav>")
	for _, want := range []string{"Active (", "Inactive (1)", "Banned (1)"} {
		if !strings.Contains(states, want) {
			t.Errorf("the state filter lacks %q: %s", want, states)
		}
	}
	if !strings.Contains(states, `aria-current=true>Active`) {
		t.Errorf("the default view does not mark Active as current: %s", states)
	}
}
