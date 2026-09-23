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
		{protocol.KindQueue, "/channels/new?kind=queue", "/channel/edit"},
		{protocol.KindPubSub, "/channels/new?kind=pubsub", "/channel/edit"},
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
		{protocol.KindAgent, "/agents/new",
			[]string{"descr", "ttl", "bound", "overflow", "allow", "personal", "maintainers", "subs"},
			[]string{"addr", "protocol", "secret"}},
		{protocol.KindService, "/services/new",
			[]string{"descr", "addr", "protocol", "secret", "allow", "maintainers", "personal"},
			[]string{"ttl", "bound", "overflow", "subs"}},
		{protocol.KindQueue, "/channels/new?kind=queue",
			[]string{"descr", "ttl", "bound", "overflow", "allow", "maintainers", "personal", "subs"},
			[]string{"addr", "protocol", "secret"}},
		{protocol.KindPubSub, "/channels/new?kind=pubsub",
			[]string{"descr", "subs", "allow", "maintainers", "personal"},
			[]string{"addr", "protocol", "secret", "ttl", "bound", "overflow"}},
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
