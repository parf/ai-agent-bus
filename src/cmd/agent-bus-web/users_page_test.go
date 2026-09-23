package main

import (
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The user directory is a list page like the others: one toolbar, one table
// of Users with a fact per column, and an empty-state card when nothing
// matches (docs/05-discovery.md#required-tabs). Each cell is read out of its own row.
func TestTheUserDirectoryShowsAFactPerColumn(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "ada@h", PersonName: "Ada", Email: "ada@example.org", GithubUser: "ada-gh"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "idle@h"}, true); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Name: "#one@h", Owner: "ada@h", Kind: protocol.KindAgent})
	m.register(protocol.Record{Name: "#two@h", Owner: "ada@h", Kind: protocol.KindAgent})
	// Not an agent: a queue ada owns is not counted as one.
	m.register(protocol.Record{Name: "jobs@h", Owner: "ada@h", Kind: protocol.KindQueue})
	// A credential with nothing behind it is not a User.
	if _, err := m.tokens.Issue("junk@h"); err != nil {
		t.Fatal(err)
	}
	body := m.get("/users")
	head := section(t, body, `<table class="record-table users-table">`, "</thead>")
	for _, col := range []string{
		"<th scope=col>User</th>", "<th scope=col>Authority</th>", "<th scope=col>Contact</th>",
		"<th scope=col class=num>Agents</th>", "<th scope=col>Last used</th>",
	} {
		if !strings.Contains(head, col) {
			t.Errorf("the directory lacks the column %s: %s", col, head)
		}
	}
	ada := m.row(body, "ada@h")
	for _, cell := range []string{
		`<td data-label=Contact><span class=contact-line>ada@example.org</span><span class=contact-line><span class=muted>GitHub</span> <code>ada-gh</code></span></td>`,
		"<td class=num data-label=Agents>2</td>",
		`<td data-label="Last used"><span class=muted>never</span></td>`,
	} {
		if !strings.Contains(ada, cell) {
			t.Errorf("ada@h's row lacks %s: %s", cell, ada)
		}
	}
	// The signed-in owner's credential made this very request.
	if admin := m.row(body, "admin@h"); !strings.Contains(admin, `<td data-label="Last used"><time datetime="`) || !strings.Contains(admin, `">now</time></td>`) {
		t.Errorf("the owner's row does not say its credential was used now: %s", admin)
	}
	if idle := m.row(body, "idle@h"); !strings.Contains(idle, "<td data-label=Contact><span class=muted>&mdash;</span></td>") || !strings.Contains(idle, "<td class=num data-label=Agents><span class=muted>0</span></td>") {
		t.Errorf("a User with no contact and no agents is not shown as such: %s", idle)
	}
	rows := section(t, body, "<tbody>", "</tbody>")
	for _, name := range []string{"junk@h", "#one@h", "jobs@h"} {
		if strings.Contains(rows, "<code>"+name+"</code>") {
			t.Errorf("the user directory lists %s, which is not a User", name)
		}
	}
	if !strings.Contains(body, "<caption>Showing 1&ndash;3 of 3 matching users.</caption>") {
		t.Errorf("the directory does not say how many Users it shows: %s", section(t, body, "<main>", "</main>"))
	}

	// An inactive User's use is not reported, so it is not called never.
	if _, err := m.bus.SetUserState("admin@h", "idle@h", protocol.StatusInactive); err != nil {
		t.Fatal(err)
	}
	if idle := m.row(m.get("/users?state=inactive"), "idle@h"); !strings.Contains(idle, `<td data-label="Last used"><span class=muted title="Not reported while inactive">&mdash;</span></td>`) {
		t.Errorf("an inactive User's last use is claimed rather than unknown: %s", idle)
	}
}

func TestASearchThatMatchesNoUserSaysSo(t *testing.T) {
	m := meaningFixture(t)
	body := m.get("/users?q=nobody-by-this-name")
	card := section(t, body, `<section class="empty-state editor-card">`, "</section>")
	if !strings.Contains(card, "<h2>No user matches this search</h2>") || !strings.Contains(card, `<a href=/users>Clear the search</a>`) {
		t.Errorf("the empty search has no card saying so: %s", card)
	}
	if strings.Contains(body, `<table class="record-table users-table">`) {
		t.Error("an empty search still draws the table")
	}
	if full := m.get("/users"); strings.Contains(full, `<section class="empty-state editor-card">`) {
		t.Error("the directory shows the empty card beside its users")
	}
}
