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
		// Not questions: the kind is fixed by the address a registration
		// came from, and a flag says whether a form carried a field at all.
		case "kind", "action", "return", "edit_allow", "edit_subs", "edit_sharing", "edit_personal":
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
		{protocol.KindAgent, "/agents/new",
			[]string{"descr", "ttl", "bound", "overflow", "allow", "personal", "maintainers"},
			[]string{"addr", "protocol", "secret", "subs"}},
		{protocol.KindService, "/services/new",
			[]string{"descr", "addr", "protocol", "secret", "allow", "maintainers"},
			[]string{"ttl", "bound", "overflow", "subs", "personal"}},
		{protocol.KindQueue, "/channels/new?kind=queue",
			[]string{"descr", "ttl", "bound", "overflow", "allow", "maintainers"},
			[]string{"addr", "protocol", "secret", "subs", "personal"}},
		{protocol.KindPubSub, "/channels/new?kind=pubsub",
			[]string{"descr", "subs", "allow", "maintainers"},
			[]string{"addr", "protocol", "secret", "ttl", "bound", "overflow", "personal"}},
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
