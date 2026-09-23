package main

import (
	"net/url"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The element each editable field is, as the settings form writes it. Asked
// of the element rather than of the word: every page carries the field
// names in its help text and stylesheet.
var fieldElement = map[string]string{
	"descr":       `<input name=descr `,
	"addr":        `<input name=addr `,
	"protocol":    `<input name=protocol `,
	"secret":      `<textarea name=secret `,
	"ttl":         `<input name=ttl `,
	"bound":       `<input type=number min=0 name=bound `,
	"overflow":    `<select name=overflow>`,
	"subs-list":   `<textarea name=subs `,
	"subs-slot":   `<input name=subs `,
	"allow":       `<textarea name=allow `,
	"personal":    `<input type=checkbox name=personal `,
	"maintainers": `<textarea name=maintainers `,
	"members":     `<textarea name=members `,
}

// Every kind's settings form offers every field the daemon lets that kind's
// manager change, and none it refuses for the kind
// (docs/constitution.md#common-record-fields).
func TestEverySettingsFormOffersEveryEditableField(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "#bot@h", Owner: "admin@h", Kind: protocol.KindAgent})
	m.register(protocol.Record{Name: "db@h", Owner: "admin@h", Kind: protocol.KindService, Addr: "h:1", Proto: "https"})
	m.register(protocol.Record{Name: "jobs@h", Owner: "admin@h", Kind: protocol.KindQueue})
	m.register(protocol.Record{Name: "news@h", Owner: "admin@h", Kind: protocol.KindPubSub})
	if err := m.bus.SetGroup("admin@h", "@crew", []string{"admin@h"}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		what, path       string
		present, missing []string
	}{
		{"agent", "/agent/edit?name=%23bot@h",
			[]string{"descr", "ttl", "bound", "overflow", "subs-slot", "allow", "personal", "maintainers", "secret"},
			[]string{"addr", "protocol", "subs-list", "members"}},
		{"service", "/service/edit?name=db@h",
			[]string{"descr", "addr", "protocol", "secret", "allow", "personal", "maintainers"},
			[]string{"ttl", "bound", "overflow", "subs-slot", "subs-list", "members"}},
		{"queue", "/queue/edit?name=jobs@h",
			[]string{"descr", "ttl", "bound", "overflow", "subs-slot", "allow", "personal", "maintainers"},
			[]string{"addr", "protocol", "secret", "subs-list", "members"}},
		{"pubsub", "/pubsub/topic/edit?name=news@h",
			[]string{"descr", "subs-list", "allow", "personal", "maintainers"},
			[]string{"addr", "protocol", "secret", "ttl", "bound", "overflow", "subs-slot", "members"}},
		// A user's own record is the user's inbox: a description and its
		// queue policy, and no allow list, Maintainers or Personal switch —
		// the daemon refuses the first two on it and fixes the third.
		{"user record", "/queue/edit?name=admin@h",
			[]string{"descr", "ttl", "bound", "overflow"},
			[]string{"allow", "maintainers", "personal", "secret", "subs-slot", "subs-list", "addr", "protocol"}},
		{"group", "/group/edit?name=%40crew",
			[]string{"descr", "members", "personal", "maintainers", "secret"},
			[]string{"allow", "ttl", "bound", "overflow", "subs-slot", "subs-list", "addr", "protocol"}},
	} {
		form := editorOf(t, m.get(c.path))
		for _, field := range c.present {
			if !strings.Contains(form, fieldElement[field]) {
				t.Errorf("the %s settings form lacks %s (%s)", c.what, field, fieldElement[field])
			}
		}
		for _, field := range c.missing {
			if strings.Contains(form, fieldElement[field]) {
				t.Errorf("the %s settings form offers %s, which the daemon refuses for it", c.what, field)
			}
		}
	}
	// And the owner is offered them enabled, with the flags that say the form
	// carried them.
	group := editorOf(t, m.get("/group/edit?name=%40crew"))
	for _, want := range []string{`<input type=hidden name=edit_sharing value=1>`, `<input type=hidden name=edit_personal value=1>`, `<textarea name=maintainers rows=5 aria-invalid`, `<textarea name=secret rows=4 autocomplete=off spellcheck=false placeholder="TOKEN=..." >`} {
		if !strings.Contains(group, want) {
			t.Errorf("the group owner's form lacks %s: %s", want, group)
		}
	}
	// Registering a group asks what editing one does but Maintainers.
	fresh := section(t, m.get("/groups/new"), "<form id=form-save", "</form>")
	for _, field := range []string{"descr", "members", "personal", "secret"} {
		if !strings.Contains(fresh, fieldElement[field]) {
			t.Errorf("group registration lacks %s", field)
		}
	}
	// An agent's registration asks for its secret too.
	if !strings.Contains(m.get("/agents/new"), fieldElement["secret"]) {
		t.Error("agent registration offers no secret")
	}
}

// What the new fields submit reaches the daemon.
func TestNewlyOfferedFieldsReachTheDaemon(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "mate@h"}, true); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Name: "#bot@h", Owner: "admin@h", Kind: protocol.KindAgent})

	// An agent's secret, on its settings form and at registration. Only the
	// agent itself reads it back.
	m.post(t, "/service", url.Values{"action": {"save"}, "name": {"#bot@h"}, "descr": {""}, "ttl": {""}, "bound": {"0"}, "overflow": {"strict"}, "secret": {"BOT=one"}}, 303)
	if got, err := m.bus.Secret("#bot@h", "#bot@h"); err != nil || got != "BOT=one" {
		t.Errorf("the agent's secret was not stored: %q %v", got, err)
	}
	m.post(t, "/service", url.Values{"action": {"create"}, "kind": {protocol.KindAgent}, "name": {"#fresh@h"}, "allow": {""}, "secret": {"FRESH=two"}}, 303)
	if got, err := m.bus.Secret("#fresh@h", "#fresh@h"); err != nil || got != "FRESH=two" {
		t.Errorf("a registered agent's secret was not stored: %q %v", got, err)
	}

	// A user's own inbox: its description and queue policy.
	m.post(t, "/service", url.Values{"action": {"save"}, "name": {"admin@h"}, "descr": {"my inbox"}, "ttl": {"1h"}, "bound": {"7"}, "overflow": {"ring"}}, 303)
	if r, _ := m.bus.Lookup("admin@h", "admin@h"); r.Descr != "my inbox" || r.TTL != "1h" || r.Bound != 7 || r.Full != protocol.OverflowRing {
		t.Errorf("the user record's settings were not stored: %+v", r)
	}

	// A group: description, Maintainers and secret in one save, members kept.
	if err := m.bus.SetGroup("admin@h", "@crew", []string{"admin@h"}); err != nil {
		t.Fatal(err)
	}
	m.post(t, "/groups", url.Values{"action": {"save"}, "name": {"@crew"}, "members": {"admin@h\r\nmate@h"}, "descr": {"The crew"},
		"edit_sharing": {"1"}, "maintainers": {"mate@h"}, "edit_personal": {"1"}, "secret": {"CREW=three"}}, 303)
	crew, _ := m.bus.Lookup("admin@h", "@crew")
	if crew.Descr != "The crew" || strings.Join(crew.Maintainers, " ") != "mate@h" || strings.Join(crew.Allow, " ") != "admin@h mate@h" || crew.Personal {
		t.Errorf("the group save did not store description, Maintainers and members: %+v", crew)
	}
	if got, err := m.bus.Secret("@crew", "mate@h"); err != nil || got != "CREW=three" {
		t.Errorf("the group's secret was not stored: %q %v", got, err)
	}
	// Personal, which a Personal group's cohort then has to fit, and which
	// only a group named for its owner may be (docs/constitution.md#-group).
	m.post(t, "/groups", url.Values{"action": {"save"}, "name": {"@crew"}, "members": {"admin@h"}, "descr": {"The crew"},
		"edit_sharing": {"1"}, "maintainers": {""}, "edit_personal": {"1"}, "personal": {"on"}}, 400)
	if err := m.bus.SetGroup("admin@h", "@admin@h/crew", []string{"admin@h"}); err != nil {
		t.Fatal(err)
	}
	m.post(t, "/groups", url.Values{"action": {"save"}, "name": {"@admin@h/crew"}, "members": {"admin@h"}, "descr": {"Mine"},
		"edit_sharing": {"1"}, "maintainers": {""}, "edit_personal": {"1"}, "personal": {"on"}}, 303)
	if own, _ := m.bus.Lookup("admin@h", "@admin@h/crew"); !own.Personal || own.Descr != "Mine" {
		t.Errorf("the group's Personal choice was not stored: %+v", own)
	}
	m.post(t, "/groups", url.Values{"action": {"save"}, "name": {"@crew"}, "members": {"admin@h\nmate@h"}, "descr": {"The crew"},
		"edit_sharing": {"1"}, "maintainers": {"mate@h"}, "edit_personal": {"1"}}, 303)

	// A Maintainer is offered the description, members and secret, and the
	// owner-only fields only as reference; its save leaves them alone.
	mate := m.as("mate@h")
	form := editorOf(t, mate.get("/group/edit?name=%40crew"))
	if !strings.Contains(form, `<input type=checkbox name=personal disabled`) || !strings.Contains(form, `<textarea name=maintainers rows=5 disabled`) || strings.Contains(form, `name=edit_sharing`) || strings.Contains(form, `name=secret rows=4 autocomplete=off spellcheck=false placeholder="TOKEN=..." disabled`) {
		t.Errorf("a Maintainer's group form offers the wrong controls: %s", form)
	}
	mate.post(t, "/groups", url.Values{"action": {"save"}, "name": {"@crew"}, "members": {"admin@h\nmate@h"}, "descr": {"Renamed by mate"}}, 303)
	if crew, _ = m.bus.Lookup("admin@h", "@crew"); crew.Descr != "Renamed by mate" || strings.Join(crew.Maintainers, " ") != "mate@h" {
		t.Errorf("a Maintainer's save did not keep the Maintainers it was not offered: %+v", crew)
	}

	// Registration carries the description, Personal and the secret.
	// An unprefixed Personal registration is refused before anything exists.
	body, _ := m.post(t, "/groups", url.Values{"action": {"save"}, "new": {"1"}, "name": {"@fresh"}, "members": {"admin@h"}, "edit_personal": {"1"}, "personal": {"on"}}, 400)
	if !strings.Contains(body, "call it @admin@h/&lt;name&gt;") {
		t.Errorf("the refusal does not say how to name it: %s", body)
	}
	if _, known := m.bus.Lookup("admin@h", "@fresh"); known {
		t.Error("a refused Personal registration left a group behind")
	}
	m.post(t, "/groups", url.Values{"action": {"save"}, "new": {"1"}, "name": {"@admin@h/fresh"}, "members": {"admin@h"}, "descr": {"Fresh"}, "edit_personal": {"1"}, "personal": {"on"}, "secret": {"FRESH=four"}}, 303)
	if fresh, _ := m.bus.Lookup("admin@h", "@admin@h/fresh"); fresh.Descr != "Fresh" || !fresh.Personal {
		t.Errorf("a registered group lost its description or Personal: %+v", fresh)
	}
	if got, err := m.bus.Secret("@admin@h/fresh", "admin@h"); err != nil || got != "FRESH=four" {
		t.Errorf("a registered group's secret was not stored: %q %v", got, err)
	}

	// The protected group takes a description from the daemon Owner, and its
	// form offers no owner-only field it does not have.
	admins := editorOf(t, m.get("/group/edit?name=%40administrators"))
	if !strings.Contains(admins, `<textarea name=maintainers rows=5 disabled`) || strings.Contains(admins, `name=edit_sharing`) || strings.Contains(admins, `name=edit_personal`) {
		t.Errorf("the protected group's form offers Maintainers or Personal: %s", admins)
	}
	m.post(t, "/groups", url.Values{"action": {"save"}, "name": {core.AdministratorsGroup}, "members": {"admin@h"}, "descr": {"Node admins"}}, 303)
	if r, _ := m.bus.Lookup("admin@h", core.AdministratorsGroup); r.Descr != "Node admins" {
		t.Errorf("the protected group's description was not stored: %+v", r)
	}

	// A group's owner reaches its Danger Zone, and a transfer made there
	// changes its owner and returns to the group.
	detail := m.get("/group?name=%40crew")
	if !strings.Contains(detail, `<a class=danger href="/service-danger?name=%40crew">Danger Zone</a>`) || !strings.Contains(detail, `<p class=group-description>Renamed by mate</p>`) {
		t.Errorf("the group page offers no Danger Zone or shows no description: %s", detail)
	}
	danger := m.get("/service-danger?name=%40crew")
	if !strings.Contains(danger, "<h2>Transfer ownership</h2>") || !strings.Contains(danger, "<h2>Replace configuration</h2>") || strings.Contains(danger, "<h2>Remove registration</h2>") || !strings.Contains(danger, `href="/group?name=%40crew"`) {
		t.Errorf("the group Danger Zone offers the wrong actions: %s", danger)
	}
	if strings.Contains(m.get("/service-danger?name=%40administrators"), "<h2>Transfer ownership</h2>") {
		t.Error("the protected group offers a transfer the daemon refuses")
	}
	_, header := m.post(t, "/service", url.Values{"action": {"transfer"}, "name": {"@crew"}, "owner": {"mate@h"}, "confirmed": {"1"}, "expected_owner": {"admin@h"}}, 303)
	if crew, _ = m.bus.Lookup("admin@h", "@crew"); crew.Owner != "mate@h" || header.Get("Location") != "/group?name=%40crew" {
		t.Errorf("the group transfer did not land: owner %q, back to %q", crew.Owner, header.Get("Location"))
	}
}
