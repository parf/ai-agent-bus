package main

import (
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/display"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A node has one daemon owner and any number of users, so a row that says only
// "User" says the least useful thing about the one identity that cannot be
// delegated. Two people are registered here on purpose: with a single user
// record, a page that marked every row would pass every assertion below.
func TestTheDaemonOwnerIsMarkedWhereARowWouldOtherwiseSayUser(t *testing.T) {
	m := meaningFixture(t)
	// admin@h is this node's daemon owner and already answers for its own
	// name, so it gets a profile rather than a second registration.
	for name, create := range map[string]bool{"admin@h": false, "other@h": true} {
		if _, err := m.bus.SetUser("admin@h", protocol.User{Name: name}, create); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	owned := display.DaemonOwnerGlyph + " Daemon owner"
	user := display.Entity(protocol.KindUser)

	// A person's own inbox is Personal, so it is on no shared listing: the
	// Queues page does not show it (docs/03-records.md#personal-and-shared).
	// Asked of other@h: the signed-in owner's name is in every page header.
	if queues := m.get("/queues"); strings.Contains(queues, "other@h") {
		t.Errorf("a user's own record is on the shared Queues page")
	}
	// The detail page behind a user's record still names the authority.
	detail := m.get("/queue?name=admin%40h")
	if !strings.Contains(detail, "<span class=fact-pill>"+owned+"</span>") {
		t.Errorf("the daemon owner's record page does not name the authority: %s", section(t, detail, "<div class=detail-meta>", "</div>"))
	}

	// And so does the directory, where a compact row carries the glyph alone
	// and the word moves into the accessible label.
	//
	// Asked of the glyph in its own position, not of the row: the row already
	// carries the mark twice over without this change — once in the Authority
	// column beside it, and once inside the accessible label, which is built
	// from the label rather than the glyph. A row-wide search passed with the
	// glyph left unmarked.
	people := m.get("/users")
	ownerRow, otherRow := row(t, people, "admin@h"), row(t, people, "other@h")
	marked := `aria-label="` + owned + `">` + display.DaemonOwnerGlyph + "<"
	plain := `aria-label="` + user + `">` + display.EntityGlyph(protocol.KindUser) + "<"
	if !strings.Contains(ownerRow, marked) {
		t.Errorf("the directory row's glyph does not mark the daemon owner: %s", ownerRow)
	}
	if !strings.Contains(otherRow, plain) {
		t.Errorf("an ordinary person's directory glyph is not the entity glyph: %s", otherRow)
	}
	if strings.Contains(otherRow, display.DaemonOwnerGlyph) {
		t.Errorf("the directory marks an ordinary person as the daemon owner: %s", otherRow)
	}
}

// row is one <tr> of a rendered table, chosen by a name inside it. A listing
// paginates and a page carries the signed-in name in its header, so a check
// that searched the whole document would pass on the wrong row.
func row(t *testing.T, page, name string) string {
	t.Helper()
	// [1:] drops everything before the first row. The signed-in name is in the
	// header of every page here, so searching from the start of the document
	// returned the whole page and every assertion below passed on it.
	for _, tr := range strings.Split(page, "<tr>")[1:] {
		if strings.Contains(tr, name) {
			return tr
		}
	}
	t.Fatalf("no row names %s in %s", name, page)
	return ""
}
