package main

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// controls is every named control inside the one form on a page, in the
// order the form asks for them. Anything outside the form is deliberately
// excluded: the help popovers beside it name the same fields in prose.
var namedControl = regexp.MustCompile(`<(?:input|textarea|select)[^>]*\bname=([a-z_]+)`)

func controls(t *testing.T, page string) []string {
	t.Helper()
	at := strings.Index(page, "<form id=form-")
	if at < 0 {
		t.Fatal("the page has no form")
	}
	end := strings.Index(page[at:], "</form>")
	if end < 0 {
		t.Fatal("the form has no end")
	}
	var out []string
	for _, m := range namedControl.FindAllStringSubmatch(page[at:at+end], -1) {
		switch m[1] {
		// Not questions: the kind and `new` are fixed by the address a
		// registration came from, and an edit_ flag says whether a form
		// carried a field at all rather than asking for one.
		case "kind", "new", "action", "return", "edit_allow", "edit_subs", "edit_sharing", "edit_personal":
			continue
		}
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

// Registering a record and changing its settings are one form. The two used
// to be separate markup and drifted field by field, which is why this asks
// the pages rather than the template: the settings page offered a queue
// policy to kinds with no queue and no credential to the one kind that has
// one, and the registration page asked for neither Maintainers nor an
// agent's inbox policy.
// See Plans/MVP/web/forms.md#rules.
func TestEveryKindAsksTheSameQuestionsToRegisterAndToEdit(t *testing.T) {
	m := meaningFixture(t)
	for _, c := range []struct {
		kind, new, edit string
	}{
		{protocol.KindAgent, "/agents/new", "/agent/edit"},
		{protocol.KindService, "/services/new", "/service/edit"},
		{protocol.KindQueue, "/queues/new", "/queue/edit"},
		{protocol.KindPubSub, "/pubsub/new", "/pubsub/topic/edit"},
	} {
		name := c.kind + "-form@h"
		if c.kind == protocol.KindAgent {
			name = "#" + name // an agent's name begins with #
		}
		record := protocol.Record{Name: name, Kind: c.kind, Owner: "admin@h"}
		if c.kind == protocol.KindService {
			record.Addr, record.Proto = "host:1", "https"
		}
		m.register(record)
		asked := controls(t, m.get(c.new))
		offered := controls(t, m.get(c.edit+"?name="+url.QueryEscape(name)))
		// Maintainers are the one question only the settings form asks: a
		// registration cannot carry them. TestRegistrationDoesNotAskForMaintainers
		// holds both sides of that exception.
		offered = without(offered, "maintainers")
		if strings.Join(asked, " ") != strings.Join(offered, " ") {
			t.Errorf("a %s is registered with %v and edited with %v", c.kind, asked, offered)
		}
		if len(asked) < 3 {
			t.Errorf("the %s form asks almost nothing (%v), so this check proves little", c.kind, asked)
		}
	}
}

// And the questions are the kind's own. A field that every form carried
// would make the check above pass while saying nothing.
func TestAFormAsksOnlyWhatItsKindHas(t *testing.T) {
	m := meaningFixture(t)
	for _, c := range []struct {
		kind    string
		path    string
		present []string
		absent  []string
	}{
		// Every kind may be Personal from 0.7 (docs/03-records.md#personal-and-shared),
		// and an agent and a queue have a one-slot Deliver-To route
		// (docs/constitution.md#-channels); a service has neither a queue nor a route.
		// An agent holds a secret of its own, which only it reads back
		// (docs/constitution.md#-private-values).
		{protocol.KindAgent, "/agents/new",
			[]string{"descr", "ttl", "bound", "overflow", "allow", "personal", "subs", "secret"},
			[]string{"addr", "protocol", "maintainers"}},
		{protocol.KindService, "/services/new",
			[]string{"descr", "addr", "protocol", "secret", "allow", "personal"},
			[]string{"ttl", "bound", "overflow", "subs", "maintainers"}},
		{protocol.KindQueue, "/queues/new",
			[]string{"descr", "ttl", "bound", "overflow", "allow", "personal", "subs"},
			[]string{"addr", "protocol", "secret", "maintainers"}},
		{protocol.KindPubSub, "/pubsub/new",
			[]string{"descr", "subs", "allow", "personal"},
			[]string{"addr", "protocol", "secret", "ttl", "bound", "overflow", "maintainers"}},
	} {
		asked := " " + strings.Join(controls(t, m.get(c.path)), " ") + " "
		for _, want := range c.present {
			if !strings.Contains(asked, " "+want+" ") {
				t.Errorf("the %s form does not ask for %s: %v", c.kind, want, asked)
			}
		}
		for _, unwanted := range c.absent {
			if strings.Contains(asked, " "+unwanted+" ") {
				t.Errorf("the %s form asks for %s, which a %s does not have: %v", c.kind, unwanted, c.kind, asked)
			}
		}
	}
}

// A registration cannot carry Maintainers — the daemon drops them — so a
// registration form that asked for them would take an answer and discard it.
// The settings form of the record it made still asks, for its Owner.
func TestRegistrationDoesNotAskForMaintainers(t *testing.T) {
	m := meaningFixture(t)
	for _, c := range []struct{ kind, new, edit string }{
		{protocol.KindAgent, "/agents/new", "/agent/edit"},
		{protocol.KindService, "/services/new", "/service/edit"},
		{protocol.KindQueue, "/queues/new", "/queue/edit"},
		{protocol.KindPubSub, "/pubsub/new", "/pubsub/topic/edit"},
	} {
		name := c.kind + "-maint@h"
		if c.kind == protocol.KindAgent {
			name = "#" + name
		}
		record := protocol.Record{Name: name, Kind: c.kind, Owner: "admin@h"}
		if c.kind == protocol.KindService {
			record.Addr, record.Proto = "host:1", "https"
		}
		m.register(record)
		registration := m.get(c.new)
		if strings.Contains(registration, "name=maintainers") || strings.Contains(registration, "name=edit_sharing") {
			t.Errorf("the %s registration form asks for Maintainers a registration cannot carry", c.kind)
		}
		settings := m.get(c.edit + "?name=" + url.QueryEscape(name))
		if !strings.Contains(settings, "<textarea name=maintainers rows=5") || !strings.Contains(settings, "name=edit_sharing value=1") {
			t.Errorf("the %s settings form no longer asks its Owner for Maintainers", c.kind)
		}
	}
}

func without(list []string, drop string) []string {
	var out []string
	for _, v := range list {
		if v != drop {
			out = append(out, v)
		}
	}
	return out
}

// A user and a group are the same rule: the form that adds one and the form
// that changes one are the same form, on pages of their own. Both were
// separate markup in one template and had already drifted — the user form in
// what it said about the GitHub login, the group form in how much of the
// membership list it showed.
// See Plans/MVP/web/forms.md#rules.
func TestAUserAndAGroupAreAddedAndEditedByTheSameForm(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "person@h", PersonName: "A Person"}, true); err != nil {
		t.Fatal(err)
	}
	if err := m.bus.SetGroup("admin@h", "@crew", []string{"admin@h"}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ what, new, edit string }{
		{"user", "/users/new", "/user/edit?name=person@h"},
		{"group", "/groups/new", "/group/edit?name=%40crew"},
	} {
		asked := controls(t, m.get(c.new))
		offered := controls(t, m.get(c.edit))
		// A group's Maintainers are set once it exists, as a record's are.
		if c.what == "group" {
			if !strings.Contains(" "+strings.Join(offered, " ")+" ", " maintainers ") {
				t.Errorf("the group settings form does not ask for Maintainers: %v", offered)
			}
			offered = without(offered, "maintainers")
		}
		if strings.Join(asked, " ") != strings.Join(offered, " ") {
			t.Errorf("a %s is added with %v and edited with %v", c.what, asked, offered)
		}
		if len(asked) < 2 {
			t.Errorf("the %s form asks almost nothing (%v), so this check proves little", c.what, asked)
		}
	}

	// And the values come back: a form that asked the right questions with
	// nothing in them would pass the comparison above.
	if page := m.get("/user/edit?name=person@h"); !strings.Contains(page, `name=person_name value="A Person"`) {
		t.Errorf("the profile form does not carry the stored person name: %s", page)
	}
	if page := m.get("/group/edit?name=%40crew"); !strings.Contains(page, ">admin@h</textarea>") {
		t.Errorf("the group form does not carry the stored membership: %s", page)
	}
}
