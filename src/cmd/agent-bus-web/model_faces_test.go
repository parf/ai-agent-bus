package main

import (
	"net/url"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The web half of K.16 (Plans/MVP/0.7.0-TODO.md): line-list refusals name
// their line, Personal is every kind's, a one-slot route says whether it is
// usable, and any User may create a Group.

// groupRow is the one table row naming group, from <tr> to </tr>: a page-wide
// search would find the signed-in name in the header whatever the row says.
func groupRow(t *testing.T, page, group string) string {
	t.Helper()
	at := strings.Index(page, `<a href="/group?name=`+url.QueryEscape(group)+`">`)
	if at < 0 {
		t.Fatalf("no row for %s: %s", group, page)
	}
	start := strings.LastIndex(page[:at], "<tr>")
	end := strings.Index(page[at:], "</tr>")
	return page[start : at+end]
}

// A refusal about one typed term is reported at the line holding it, with the
// daemon's own message and every line kept as typed — never retyped with the
// "#" the daemon says it lacks, and never a guess at a kind.
func TestARefusedLineListNamesTheLineItRefused(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "#worker@h", Owner: "admin@h", Allow: []string{"*"}})
	m.register(protocol.Record{Name: "#svc@h", Owner: "admin@h"})
	save := func(extra url.Values) url.Values {
		form := url.Values{"action": {"save"}, "name": {"#svc@h"}, "descr": {""}, "ttl": {""}, "bound": {"0"}, "overflow": {"strict"}}
		for k, v := range extra {
			form[k] = v
		}
		return form
	}

	// The allow list: line three, counted as the textarea shows it (CRLF).
	body, _ := m.post(t, "/service", save(url.Values{"edit_allow": {"1"}, "allow": {"*\r\n@owner\r\nworker@h"}}), 400)
	if !strings.Contains(body, "<p>Line 3: bad name: worker@h names no user; an agent&#39;s name begins with #, so the agent is #worker@h</p>") {
		t.Fatalf("the allow refusal does not name line 3 with the daemon's message: %s", body)
	}
	editor := editorOf(t, body)
	if !strings.Contains(editor, `aria-invalid="true" aria-describedby="save-error">* @owner worker@h</textarea>`) {
		t.Fatalf("the allow field was not marked, or its lines were not kept as typed: %s", editor)
	}
	if r, _ := m.bus.Lookup("admin@h", "#svc@h"); len(r.Allow) != 0 {
		t.Fatalf("a refused save stored %v", r.Allow)
	}

	// Maintainers: the daemon's message names its list, so the term typed in
	// both lists is reported in that one alone.
	body, _ = m.post(t, "/service", save(url.Values{
		"edit_allow": {"1"}, "allow": {"*\n@nosuch"},
		"edit_sharing": {"1"}, "maintainers": {"admin@h\n@nosuch"},
	}), 404)
	if !strings.Contains(body, "<p>Line 2: ") || !strings.Contains(body, "maintainer @nosuch</p>") || strings.Contains(body, "Allow list line") {
		t.Fatalf("the maintainer refusal does not name line 2: %s", body)
	}
	if !strings.Contains(editorOf(t, body), `<textarea name=maintainers rows=5 aria-invalid="true"`) {
		t.Fatalf("the Maintainers field was not marked: %s", editorOf(t, body))
	}

	// A term in two lists, with a message naming neither list, is reported in
	// both rather than in whichever was guessed.
	body, _ = m.post(t, "/service", save(url.Values{
		"edit_allow": {"1"}, "allow": {"*\nworker@h"},
		"edit_sharing": {"1"}, "maintainers": {"worker@h"},
	}), 400)
	if !strings.Contains(body, "Allow list line 2 and Maintainers line 1: bad name: worker@h names no user") {
		t.Fatalf("an ambiguous refusal was pinned on one list: %s", body)
	}

	// A refusal naming no typed line claims none.
	body, _ = m.post(t, "/service", save(url.Values{"edit_allow": {"1"}, "allow": {"*"}, "overflow": {"sideways"}}), 400)
	if strings.Contains(body, "Line ") {
		t.Fatalf("a refusal about no line was given one: %s", body)
	}

	// Registration: the same form, the same report.
	body, _ = m.post(t, "/service", url.Values{"action": {"create"}, "kind": {"queue"}, "name": {"jobs@h"}, "allow": {"@owner\nworker@h"}}, 400)
	if !strings.Contains(body, "<p>Line 2: bad name: worker@h names no user") || !strings.Contains(body, `aria-invalid="true" aria-describedby="create-error">@owner worker@h</textarea>`) {
		t.Fatalf("the registration form did not name the refused line: %s", body)
	}

	// Group members, on registration: the members line is marked, not the name.
	body, _ = m.post(t, "/groups", url.Values{"action": {"save"}, "new": {"1"}, "name": {"@crew"}, "members": {"admin@h\nworker@h"}}, 400)
	if !strings.Contains(body, "<p>Line 2: bad name: worker@h names no user") {
		t.Fatalf("the group refusal does not name line 2: %s", body)
	}
	if !strings.Contains(body, `<textarea name=members rows=8 placeholder="user@realm&#10;#agent@realm&#10;@nested-group" aria-invalid="true"`) || !strings.Contains(body, `name=name placeholder="@operators" required value="@crew" aria-invalid="false"`) {
		t.Fatalf("the group form marked the wrong field: %s", body)
	}
	if !strings.Contains(body, ">admin@h worker@h</textarea>") {
		t.Fatalf("the typed members were not kept: %s", body)
	}
}

// Personal is every kind's but a user's own record, and only its Owner (or
// the daemon Owner) sets it; a Maintainer sees the control disabled.
// See docs/03-records.md#personal-and-shared.
func TestPersonalIsOfferedOnEveryKindToItsOwner(t *testing.T) {
	p := personalWebFixture(t)
	if _, err := p.bus.Register(protocol.Record{Name: "alice-q@h", Kind: protocol.KindQueue, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	maintainers := protocol.MaintainerList{"bob@h"}
	if _, err := p.bus.Manage("alice@h", core.Management{Name: "alice-q@h", Maintainers: &maintainers}); err != nil {
		t.Fatal(err)
	}
	owner, _ := p.request("alice@h", "GET", "/channel/edit?name=alice-q@h", nil, 200)
	if !strings.Contains(owner, "<input type=hidden name=edit_personal value=1>") || !strings.Contains(owner, "<input type=checkbox name=personal  >") {
		t.Fatalf("the queue's Owner is not offered Personal: %s", owner)
	}
	maintainer, _ := p.request("bob@h", "GET", "/channel/edit?name=alice-q@h", nil, 200)
	if strings.Contains(maintainer, "name=edit_personal") || !strings.Contains(maintainer, "<input type=checkbox name=personal disabled >") {
		t.Fatalf("a Maintainer was offered Personal: %s", maintainer)
	}

	// The Owner clears the sharing and sets Personal in one save.
	p.request("alice@h", "POST", "/service", url.Values{
		"action": {"save"}, "name": {"alice-q@h"}, "descr": {""}, "ttl": {""}, "bound": {"0"}, "overflow": {"strict"},
		"edit_allow": {"1"}, "allow": {""}, "edit_sharing": {"1"}, "maintainers": {""},
		"edit_personal": {"1"}, "personal": {"on"},
	}, 303)
	if r, _ := p.bus.Lookup("alice@h", "alice-q@h"); !r.Personal {
		t.Fatal("the Owner's Personal choice was not saved")
	}
	channels, _ := p.request("alice@h", "GET", "/channels", nil, 200)
	personal, _ := p.request("alice@h", "GET", "/personal", nil, 200)
	if strings.Contains(channels, `<code class=record-description>alice-q@h</code>`) || !strings.Contains(personal, `<code class=record-description>alice-q@h</code>`) {
		t.Fatalf("a Personal queue is not on the Personal page alone: %s", personal)
	}
	// The page now holds every kind, so it is named for none of them.
	if !strings.Contains(personal, "</span> Personal</h1>") || strings.Contains(personal, "Personal agents") {
		t.Fatalf("the Personal page still calls itself agents: %s", personal)
	}
	if !strings.Contains(channels, `<a href="/personal" class="personal-view">Personal (1)</a>`) {
		t.Fatalf("the Channels page offers no way to its Personal records: %s", channels)
	}
}

// A one-slot route is shown with whether the destination allows it now,
// with the record itself as the identity checked there, and never as a door
// for every sender. See docs/constitution.md#-channels.
func TestARouteSaysWhetherItIsUsableAndWhoIsChecked(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "viewer@h"}, true); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Name: "#src@h", Owner: "admin@h", Allow: []string{"viewer@h"}})
	m.register(protocol.Record{Name: "#dest@h", Owner: "admin@h", Allow: []string{"#src@h"}})
	m.register(protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "admin@h"})

	none := section(t, m.get("/channel?name=jobs@h"), "<section class=\"dashboard-section route\" id=route>", "</section>")
	if !strings.Contains(none, "No route.") || !strings.Contains(none, ">Set a route in the settings</a>") {
		t.Fatalf("a queue without a route does not say so: %s", none)
	}

	// Set through the one settings form, as a single line.
	m.post(t, "/service", url.Values{
		"action": {"save"}, "name": {"#src@h"}, "descr": {""}, "ttl": {""}, "bound": {"0"}, "overflow": {"strict"},
		"edit_allow": {"1"}, "allow": {"viewer@h"}, "edit_subs": {"1"}, "subs": {"#dest@h"},
	}, 303)
	if r, _ := m.bus.Lookup("admin@h", "#src@h"); len(r.Subs) != 1 || r.Subs[0] != "#dest@h" {
		t.Fatalf("the route was not stored: %v", r.Subs)
	}
	if editor := editorOf(t, m.get("/agent/edit?name=%23src@h")); !strings.Contains(editor, `<input name=subs value="#dest@h"`) {
		t.Fatalf("the editor does not carry the stored route back: %s", editor)
	}

	const open = "<section class=\"dashboard-section route\" id=route>"
	allowed := section(t, m.get("/agent?name=%23src@h"), open, "</section>")
	for _, want := range []string{
		"Forwards to <code>#dest@h</code>",
		"<strong>Route allowed now.</strong>",
		"is <code>#src@h</code> itself &mdash; not the original sender",
		"an allowed route admits nobody that list does not",
		">Replace or clear the route in the settings</a>",
	} {
		if !strings.Contains(allowed, want) {
			t.Errorf("the allowed route does not say %q: %s", want, allowed)
		}
	}
	if strings.Contains(allowed, "Configured, but") {
		t.Errorf("an allowed route is called unusable: %s", allowed)
	}
	// Somebody who may use the source and not manage it is told the same
	// facts and offered no control.
	if viewer := section(t, m.as("viewer@h").get("/agent?name=%23src@h"), open, "</section>"); !strings.Contains(viewer, "Route allowed now.") || strings.Contains(viewer, "in the settings</a>") {
		t.Errorf("a non-manager's view of the route is wrong: %s", viewer)
	}

	// The destination stops listing the source: still configured, not usable.
	stop := []string{"admin@h"}
	if _, err := m.bus.Manage("admin@h", core.Management{Name: "#dest@h", Allow: &stop}); err != nil {
		t.Fatal(err)
	}
	refused := section(t, m.get("/agent?name=%23src@h"), open, "</section>")
	if !strings.Contains(refused, "Configured, but <code>#dest@h</code> does not allow <code>#src@h</code> now") || strings.Contains(refused, "Route allowed now.") {
		t.Fatalf("a route the destination no longer allows is not told apart: %s", refused)
	}

	// A route the destination refuses to store is a refusal of that line.
	body, _ := m.post(t, "/service", url.Values{
		"action": {"save"}, "name": {"jobs@h"}, "descr": {""}, "ttl": {""}, "bound": {"0"}, "overflow": {"strict"},
		"edit_allow": {"1"}, "allow": {""}, "edit_subs": {"1"}, "subs": {"#dest@h"},
	}, 403)
	if !strings.Contains(body, "Line 1: ") || !strings.Contains(body, "#dest@h does not allow jobs@h") || !strings.Contains(body, `<input name=subs value="#dest@h" placeholder="#agent@realm, queue@realm or topic@realm" aria-invalid="true"`) {
		t.Fatalf("a refused route did not come back as its field: %s", body)
	}

	// Cleared by writing the field empty.
	m.post(t, "/service", url.Values{
		"action": {"save"}, "name": {"#src@h"}, "descr": {""}, "ttl": {""}, "bound": {"0"}, "overflow": {"strict"},
		"edit_allow": {"1"}, "allow": {"viewer@h"}, "edit_subs": {"1"}, "subs": {""},
	}, 303)
	if r, _ := m.bus.Lookup("admin@h", "#src@h"); len(r.Subs) != 0 {
		t.Fatalf("clearing the route left %v", r.Subs)
	}
}

// Any User creates a Group and owns it; its Owner and Maintainers are shown
// where the caller may see the group, and @administrators stays the daemon
// Owner's. See docs/constitution.md#-group.
func TestAnOrdinaryUserCreatesAGroupAndItsOwnerIsShown(t *testing.T) {
	p := personalWebFixture(t)
	groups, _ := p.request("alice@h", "GET", "/groups", nil, 200)
	if !strings.Contains(groups, `<a href="/groups/new">Register group</a>`) {
		t.Fatalf("an ordinary user is not offered group registration: %s", groups)
	}
	p.request("alice@h", "POST", "/groups", url.Values{"action": {"save"}, "new": {"1"}, "name": {"@alice-team"}, "members": {"alice@h\nbob@h"}}, 303)

	row := groupRow(t, mustGet(p, "alice@h", "/groups"), "@alice-team")
	if !strings.Contains(row, "<td data-label=Owner><code>alice@h</code></td>") || !strings.Contains(row, "<td data-label=Maintainers><span class=muted>None</span></td>") {
		t.Fatalf("the group's Owner is not shown: %s", row)
	}
	if !strings.Contains(mustGet(p, "alice@h", "/group?name=%40alice-team"), "id=members-edit") {
		t.Fatal("the group's Owner is not offered its membership editor")
	}
	if strings.Contains(mustGet(p, "bob@h", "/group?name=%40alice-team"), "id=members-edit") {
		t.Fatal("a mere member is offered the membership editor")
	}

	// A Maintainer is shown, and may edit.
	maintainers := protocol.MaintainerList{"bob@h"}
	if _, err := p.bus.Manage("alice@h", core.Management{Name: "@alice-team", Maintainers: &maintainers}); err != nil {
		t.Fatal(err)
	}
	if row := groupRow(t, mustGet(p, "bob@h", "/groups"), "@alice-team"); !strings.Contains(row, "<td data-label=Maintainers><code>bob@h</code></td>") {
		t.Fatalf("the group's Maintainer is not shown: %s", row)
	}
	if !strings.Contains(mustGet(p, "bob@h", "/group?name=%40alice-team"), "id=members-edit") {
		t.Fatal("a Maintainer is not offered the membership editor")
	}
	p.request("bob@h", "POST", "/groups", url.Values{"action": {"save"}, "name": {"@alice-team"}, "members": {"alice@h"}}, 303)

	// @administrators: protected, and its controls are the daemon Owner's.
	admins := mustGet(p, "alice@h", "/group?name=%40administrators")
	if strings.Contains(admins, "id=members-edit") || !strings.Contains(admins, "Only the daemon owner changes this protected group.") || !strings.Contains(admins, "<span class=fact-pill>protected</span>") {
		t.Fatalf("an ordinary user was offered @administrators' controls: %s", admins)
	}
	p.request("alice@h", "GET", "/group/edit?name=%40administrators", nil, 403)
	if !strings.Contains(mustGet(p, "admin@h", "/group?name=%40administrators"), "id=members-edit") {
		t.Fatal("the daemon Owner lost @administrators' editor")
	}
}

// The term a message names is matched as a whole name, where it repeats when
// the refusal is a duplicate, and up to a sentence's closing full stop.
func TestOffendingLineFindsOnlyTheNamedTerm(t *testing.T) {
	for _, c := range []struct {
		message, text string
		want          string
	}{
		{"unknown name: deliver-to #dest@h", "dest@h", ""},
		{"unknown name: deliver-to #dest@h", "dest@h\n#dest@h", "Line 2"},
		{"bad name: duplicate maintainer bob@h", "bob@h\nalice@h\nbob@h", "Line 3"},
		{"refused: it names y@h.", "x@h\ny@h", "Line 2"},
		{"refused: it names y@h.example", "y@h", ""},
	} {
		_, got, _ := offendingLine(c.message, listField{"maintainers", "Maintainers", c.text})
		if got != c.want {
			t.Errorf("%q in %q: got %q, want %q", c.message, c.text, got, c.want)
		}
	}
}

func mustGet(p *personalWeb, who, path string) string {
	p.t.Helper()
	body, _ := p.request(who, "GET", path, nil, 200)
	return strings.Join(strings.Fields(body), " ")
}
