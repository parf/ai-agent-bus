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

	// A person's own inbox is a record, and the channels listing is where one
	// is shown. Asked of the row, not of the page: the signed-in owner's name
	// is in the header of every page here.
	channels := m.get("/channels")
	ownerRow := row(t, channels, "admin@h")
	otherRow := row(t, channels, "other@h")
	if !strings.Contains(ownerRow, owned) {
		t.Errorf("the daemon owner's row does not name the authority: %s", ownerRow)
	}
	if strings.Contains(ownerRow, user) {
		t.Errorf("the daemon owner's row still reads as one more user: %s", ownerRow)
	}
	// The other half of the same fact: a mark that is on every row is a column
	// heading, and this is the row that proves it is not.
	if !strings.Contains(otherRow, user) {
		t.Errorf("an ordinary person lost the entity label: %s", otherRow)
	}
	if strings.Contains(otherRow, display.DaemonOwnerGlyph) {
		t.Errorf("an ordinary person is marked as the daemon owner: %s", otherRow)
	}

	// The detail page behind that row says the same thing.
	detail := m.get("/channel?name=admin%40h")
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
	ownerRow, otherRow = row(t, people, "admin@h"), row(t, people, "other@h")
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

// The Channels page lists three kinds, so its Kind switch is the only thing
// that tells them apart. The daemon owner's own inbox is the case where the
// switch and the label disagree on purpose: the row is selected by the kind the
// daemon stated and labelled by the authority that outranks it, so a filter
// written against the visible label would lose the one row it most needs.
func TestTheChannelsKindSwitchSelectsByKindAndNotByTheVisibleLabel(t *testing.T) {
	m := meaningFixture(t)
	for _, record := range []protocol.Record{
		{Name: "work@h", Owner: "admin@h", Kind: protocol.KindQueue},
		{Name: "shout@h", Owner: "admin@h", Kind: protocol.KindPubSub},
	} {
		m.register(record)
	}
	for name, create := range map[string]bool{"admin@h": false, "other@h": true} {
		if _, err := m.bus.SetUser("admin@h", protocol.User{Name: name}, create); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	// Every kind the page lists is offered, each by its own label.
	all := m.get("/channels")
	for _, kind := range []string{protocol.KindUser, protocol.KindQueue, protocol.KindPubSub} {
		if !strings.Contains(all, `href="/channels?kind=`+kind+`"`) {
			t.Errorf("the Kind switch does not offer %s: %s", kind, section(t, all, `aria-label="Kind filter"`, "</nav>"))
		}
	}

	// Each selection keeps its own kind and drops the other two. Asked in both
	// directions, because a filter that returned everything would satisfy the
	// first half of every case on its own.
	for kind, kept := range map[string]string{
		protocol.KindUser:   "other@h",
		protocol.KindQueue:  "work@h",
		protocol.KindPubSub: "shout@h",
	} {
		page := m.get("/channels?kind=" + kind)
		if !strings.Contains(page, kept) {
			t.Errorf("Kind %s lost %s: %s", kind, kept, page)
		}
		for other, name := range map[string]string{
			protocol.KindUser:   "other@h",
			protocol.KindQueue:  "work@h",
			protocol.KindPubSub: "shout@h",
		} {
			if other == kind {
				continue
			}
			if strings.Contains(page, `>`+name+`</code>`) {
				t.Errorf("Kind %s also returned the %s record %s: %s", kind, other, name, page)
			}
		}
	}

	// And the row whose label is not its kind. admin@h is a user record and is
	// selected as one, while its Type cell names the authority.
	users := m.get("/channels?kind=" + protocol.KindUser)
	owner := row(t, users, "admin@h")
	if !strings.Contains(owner, display.DaemonOwnerGlyph+" Daemon owner") {
		t.Errorf("the daemon owner's row is not labelled by its authority: %s", owner)
	}
	if strings.Contains(users, `>work@h</code>`) {
		t.Errorf("Kind User returned a queue: %s", users)
	}
}
