package main

import (
	"net/url"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// editorOf is the settings form on its own page, from the form element to the
// end of it. The whole page will not do: the help popovers beside it name the
// same fields, so a page-wide search finds a field whether or not the form
// actually offers one.
func editorOf(t *testing.T, page string) string {
	t.Helper()
	at := strings.Index(page, "id=form-save")
	if at < 0 {
		t.Fatal("the page has no settings form")
	}
	end := strings.Index(page[at:], "</form>")
	if end < 0 {
		t.Fatal("the settings form has no end")
	}
	return page[at : at+end]
}

// A pub/sub topic's two lists are edited in the one record editor, so the
// Deliver-To field has to come back filled in. A save is a whole replacement:
// a form that rendered the field empty would empty the list every time
// somebody changed a description.
// See docs/04-messaging.md#subscribers.
func TestTheRecordEditorCarriesTheDeliverToListBackAndForth(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "#reader@h", Kind: protocol.KindAgent, Owner: "admin@h", Allow: []string{"*"}})
	if err := m.bus.SetGroup("admin@h", "@team", []string{"admin@h"}); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "admin@h", Descr: "News", Allow: []string{"*"}})
	if _, err := m.bus.Manage("admin@h", core.Management{Name: "news@h", Subs: &[]string{"#reader@h", "@team"}}); err != nil {
		t.Fatal(err)
	}

	editor := editorOf(t, m.get("/pubsub/topic/edit?name=news@h"))
	// The fixture collapses whitespace, so the two lines arrive as one.
	if !strings.Contains(editor, "#reader@h @team</textarea>") {
		t.Fatalf("the editor did not offer the stored list back: %s", editor)
	}
	if !strings.Contains(editor, "name=edit_subs value=1") {
		t.Fatal("the editor does not state that it carried the Deliver-To field")
	}

	// Changing something else, with the field as the page rendered it.
	m.post(t, "/service", url.Values{
		"action": {"save"}, "name": {"news@h"}, "descr": {"Newsroom"},
		"edit_allow": {"1"}, "allow": {"*"},
		"edit_subs": {"1"}, "subs": {"#reader@h\n@team"},
		"ttl": {""}, "bound": {"0"}, "overflow": {"strict"},
	}, 303)
	r, _ := m.bus.Lookup("admin@h", "news@h")
	if r.Descr != "Newsroom" {
		t.Fatalf("the description was not saved: %q", r.Descr)
	}
	if len(r.Subs) != 2 || r.Subs[0] != "#reader@h" || r.Subs[1] != "@team" {
		t.Fatalf("an unrelated save changed the Deliver-To list to %v", r.Subs)
	}

	// A save that never carried the field at all leaves the list alone. The
	// editor always renders it, so this is the older bookmarked form and the
	// hand-written post — and blank below is a different thing entirely.
	m.post(t, "/service", url.Values{
		"action": {"save"}, "name": {"news@h"}, "descr": {"Newsroom"},
		"ttl": {""}, "bound": {"0"}, "overflow": {"strict"},
	}, 303)
	if r, _ := m.bus.Lookup("admin@h", "news@h"); len(r.Subs) != 2 {
		t.Fatalf("a save that never mentioned the list left %v", r.Subs)
	}

	// And blank is a real value here: it is how a topic stops delivering.
	m.post(t, "/service", url.Values{
		"action": {"save"}, "name": {"news@h"}, "descr": {"Newsroom"},
		"edit_allow": {"1"}, "allow": {"*"},
		"edit_subs": {"1"}, "subs": {""},
		"ttl": {""}, "bound": {"0"}, "overflow": {"strict"},
	}, 303)
	if r, _ := m.bus.Lookup("admin@h", "news@h"); len(r.Subs) != 0 {
		t.Fatalf("an emptied field left %v on the list", r.Subs)
	}
}

// The field follows the kind, like every other one on this form: a 📣 has a
// Deliver-To list, an 👾 and a 📮 a one-slot route (docs/constitution.md#-channels),
// and a 📡 neither. A save from a form that never showed the field must not
// be refused for a list it could not have sent.
func TestDeliverToFollowsTheKindAndOtherKindsStillSave(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "admin@h", Descr: "Jobs", Allow: []string{"*"}})
	m.register(protocol.Record{Name: "db@h", Kind: protocol.KindService, Owner: "admin@h", Addr: "host:1", Proto: "https"})
	editor := editorOf(t, m.get("/queue/edit?name=jobs@h"))
	if strings.Contains(editor, "<textarea name=subs") || !strings.Contains(editor, "<input name=subs ") {
		t.Fatalf("a queue's editor does not offer its one-slot route as a single line: %s", editor)
	}
	if service := editorOf(t, m.get("/service/edit?name=db@h")); strings.Contains(service, "name=subs") || strings.Contains(service, "edit_subs") {
		t.Fatal("a service's editor offers a Deliver-To field")
	}
	m.post(t, "/service", url.Values{
		"action": {"save"}, "name": {"jobs@h"}, "descr": {"Job queue"},
		"edit_allow": {"1"}, "allow": {"*"},
		"ttl": {""}, "bound": {"0"}, "overflow": {"strict"},
	}, 303)
	if r, _ := m.bus.Lookup("admin@h", "jobs@h"); r.Descr != "Job queue" {
		t.Fatalf("a queue could not be saved: %q", r.Descr)
	}
}

// Registering a topic and saying who it delivers to is one act, so the form
// that creates it asks — and the created record carries what was asked for,
// not a list somebody has to add afterwards.
func TestRegisteringAPubSubTopicCarriesTheDeliverToListItDeclared(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "#reader@h", Kind: protocol.KindAgent, Owner: "admin@h", Allow: []string{"*"}})
	// The exact field, not a prefix of it: name=subs-anything contains
	// name=subs and would pass a looser search while submitting nothing.
	const field = "<textarea name=subs "
	form := m.get("/pubsub/new")
	if !strings.Contains(form, field) {
		t.Fatal("the pub/sub registration form does not ask who it delivers to")
	}
	if queue := m.get("/queues/new"); strings.Contains(queue, field) {
		t.Fatal("the queue registration form asks for a Deliver-To list rather than its one slot")
	}
	m.post(t, "/service", url.Values{
		"action": {"create"}, "kind": {"pubsub"}, "name": {"feed@h"},
		"descr": {"Feed"}, "allow": {"*"}, "subs": {"#reader@h"},
	}, 303)
	r, ok := m.bus.Lookup("admin@h", "feed@h")
	if !ok {
		t.Fatal("the topic was not registered")
	}
	if len(r.Subs) != 1 || r.Subs[0] != "#reader@h" {
		t.Fatalf("the declared Deliver-To list became %v", r.Subs)
	}
}

// Taking your own inbox off is the one thing a recipient may do to the list,
// so the control appears for a name that is on it under its own name and for
// nobody else — a name that receives through a group cannot remove itself,
// and a button that did nothing would be worse than no button.
func TestTheLeaveControlAppearsOnlyForANameOnTheListItself(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "#visitor@h", Kind: protocol.KindAgent, Owner: "admin@h"})
	m.register(protocol.Record{Name: "#grouped@h", Kind: protocol.KindAgent, Owner: "admin@h"})
	if err := m.bus.SetGroup("admin@h", "@team", []string{"#grouped@h"}); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "admin@h", Allow: []string{"*"}})
	if _, err := m.bus.Manage("admin@h", core.Management{Name: "news@h", Subs: &[]string{"#visitor@h", "@team"}}); err != nil {
		t.Fatal(err)
	}
	const leave = "value=unsubscribe"
	if page := m.as("#visitor@h").get("/pubsub/topic?name=news@h"); !strings.Contains(page, leave) {
		t.Fatal("a name on the list is not offered the way off it")
	}
	if page := m.as("#grouped@h").get("/pubsub/topic?name=news@h"); strings.Contains(page, leave) {
		t.Fatal("a name that receives through a group is offered a removal that would take nothing out")
	}
	// And it takes the name off rather than putting one on, which is the
	// only direction this control has: the two used to be one form.
	m.as("#visitor@h").post(t, "/service", url.Values{"action": {"unsubscribe"}, "name": {"news@h"}}, 303)
	r, _ := m.bus.Lookup("admin@h", "news@h")
	if len(r.Subs) != 1 || r.Subs[0] != "@team" {
		t.Fatalf("the list after the recipient left is %v", r.Subs)
	}
}
