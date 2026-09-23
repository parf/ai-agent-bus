package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func TestRetainedFormKeepsOnlyNamedSafeFields(t *testing.T) {
	form := retainedForm("save", "refused", url.Values{
		"descr": {"safe description"},
		"token": {"must never enter presentation state"},
	}, "descr")
	if form.Value("descr") != "safe description" {
		t.Fatal("named safe field was not retained")
	}
	if form.Value("token") != "" || len(form.Values) != 1 {
		t.Fatal("unrecognised field entered presentation state")
	}
}

func TestDangerZoneNeverRendersRetainedConfiguration(t *testing.T) {
	const secret = "CONFIGURATION-MUST-STAY-EMPTY"
	w := httptest.NewRecorder()
	render(w, serviceDanger, adminView{
		You: "owner@h",
		Record: protocol.Record{
			Name: "#svc@h", Owner: "owner@h", Kind: protocol.KindAgent, CanManage: true, CanTransfer: true,
		},
		Form: formState{Action: "configure", Target: "configure", Field: "config", Error: "refused", Values: map[string]string{"config": secret}},
	})
	if strings.Contains(w.Body.String(), secret) {
		t.Fatal("Danger Zone rendered retained private configuration")
	}
}

func TestDashboardOwnerControls(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "admin@h")
	if err != nil {
		t.Fatal(err)
	}
	backend := httptest.NewServer(api.New(b, tokens, "admin@h").Handler())
	defer backend.Close()
	web := httptest.NewServer(dashboard(&caller{client: backend.Client(), base: backend.URL}, false))
	defer web.Close()
	client := web.Client()
	client.CheckRedirect = func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	sessions := map[string]*http.Cookie{}
	for _, who := range []string{"owner@h", "other@h", "operator@h", "admin@h"} {
		// Registered before signing in, not after: a name the daemon holds
		// nothing for but a credential cannot sign in at all
		// (docs/02-access.md#what-a-call-carries).
		known(t, b, who)
		token, err := tokens.Issue(who)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.PostForm(web.URL+"/signin", url.Values{"token": {token}})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 303 || len(resp.Cookies()) != 1 {
			t.Fatal("sign in did not return session")
		}
		sessions[who] = resp.Cookies()[0]
	}
	if err := b.SetGroup("admin@h", core.AdministratorsGroup, []string{"admin@h", "operator@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "owner@h", Descr: "service", Allow: []string{"owner@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Configure("#svc@h", "owner@h", json.RawMessage(`{"secret":"NEVER-RENDER-THIS"}`)); err != nil {
		t.Fatal(err)
	}
	// An external one beside it: only a service carries an endpoint, so it is
	// the only record whose settings form has one to lose.
	if _, err := b.Register(protocol.Record{Kind: protocol.KindService, Name: "db@h", Owner: "owner@h", Descr: "external", Addr: "db.example:5432", Proto: "postgresql", Allow: []string{"owner@h"}}); err != nil {
		t.Fatal(err)
	}
	request := func(who, method, path, origin string, form url.Values, want int) string {
		t.Helper()
		var input io.Reader
		if form != nil {
			input = strings.NewReader(form.Encode())
		}
		req, err := http.NewRequest(method, web.URL+path, input)
		if err != nil {
			t.Fatal(err)
		}
		if form != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if who != "" {
			req.AddCookie(sessions[who])
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != want {
			t.Fatalf("%s %s: %d %s, want %d", who, path, resp.StatusCode, body, want)
		}
		if resp.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("private page may be cached")
		}
		if strings.Contains(string(body), "NEVER-RENDER-THIS") {
			t.Fatal("private configuration rendered")
		}
		for _, session := range sessions {
			if strings.Contains(string(body), session.Value) {
				t.Fatal("credential rendered")
			}
		}
		return string(body)
	}
	request("", "GET", "/services", "", nil, 401)
	page := request("owner@h", "GET", "/service?name=%23svc@h", "", nil, 200)
	if !strings.Contains(page, `>Edit settings</a>`) || !strings.Contains(page, `href="/agent/edit?name=%23svc%40h`) {
		t.Fatal("owner detail is missing the way to its settings form")
	}
	editor := request("owner@h", "GET", "/agent/edit?name=%23svc@h", "", nil, 200)
	if !strings.Contains(editor, ">Save settings</button>") || !strings.Contains(editor, `class=danger`) || !strings.Contains(editor, `>Danger Zone</a>`) {
		t.Fatal("the settings form is missing its save or its Danger Zone link")
	}
	for _, hidden := range []string{"Replace configuration", "Transfer ownership", "Remove registration", "Remove idle service"} {
		if strings.Contains(page, hidden) {
			t.Fatalf("ordinary detail exposed dangerous action %q", hidden)
		}
	}
	refusedSettings := request("owner@h", "POST", "/service", web.URL, url.Values{
		"action":     {"save"},
		"name":       {"#svc@h"},
		"descr":      {"Changed & retained"},
		"ttl":        {"2h"},
		"overflow":   {"ring"},
		"bound":      {"not-a-number"},
		"edit_allow": {"1"},
		"allow":      {"other@h\n*"},
		"token":      {"UNEXPECTED-FORM-SECRET"},
	}, 400)
	for _, retained := range []string{
		"Check this form",
		`<section class=form-error role=alert`,
		`href="#form-save"`,
		`value="Changed &amp; retained"`,
		`value="2h"`,
		`value="not-a-number" aria-invalid="true"`,
		">other@h\n*</textarea>",
	} {
		if !strings.Contains(refusedSettings, retained) {
			t.Fatalf("refused settings lost safe input %q", retained)
		}
	}
	if strings.Contains(refusedSettings, "UNEXPECTED-FORM-SECRET") {
		t.Fatal("unrecognised submitted field was reflected into the form")
	}
	// The endpoint is offered where it means something, and a refusal keeps it.
	if agent := request("owner@h", "GET", "/agent?name=%23svc@h", "", nil, 200); strings.Contains(agent, "name=addr") || strings.Contains(agent, "name=protocol") {
		t.Fatal("an agent's settings offer an endpoint, which would make it read as external")
	}
	refusedExternal := request("owner@h", "POST", "/service", web.URL, url.Values{
		"action": {"save"}, "name": {"db@h"}, "descr": {"still external"},
		"addr": {"db.example:6000"}, "protocol": {""},
	}, 400)
	for _, retained := range []string{`value="db.example:6000"`} {
		if !strings.Contains(refusedExternal, retained) {
			t.Fatalf("refused service settings lost its endpoint %q", retained)
		}
	}
	// A service has no queue, so its form asks nothing about one, and the save
	// changes only what the form showed.
	external := request("owner@h", "GET", "/service?name=db@h", "", nil, 200)
	for _, absent := range []string{"name=bound", "name=ttl", "name=overflow"} {
		if strings.Contains(external, absent) {
			t.Fatalf("the service editor offers %q, which is about a queue it does not have", absent)
		}
	}
	request("owner@h", "POST", "/service", web.URL, url.Values{
		"action": {"save"}, "name": {"db@h"}, "descr": {"external"},
		"addr": {"db.example:5432"}, "protocol": {"postgresql"},
	}, 303)
	if kept, _ := b.Lookup("owner@h", "db@h"); kept.Addr != "db.example:5432" || kept.Proto != "postgresql" || kept.Full != "" || kept.Bound != 0 {
		t.Fatalf("a service save lost its endpoint or acquired a queue setting: %+v", kept)
	}
	danger := request("owner@h", "GET", "/service-danger?name=%23svc@h", "", nil, 200)
	for _, label := range []string{"Replace configuration", "Transfer ownership", "Remove registration"} {
		if !strings.Contains(danger, label) {
			t.Fatalf("Danger Zone missing owner control: %s", label)
		}
	}
	const refusedSecret = "REFUSED-CONFIG-MUST-NOT-RETURN"
	refusedConfig := request("owner@h", "POST", "/service", web.URL, url.Values{
		"action": {"configure"}, "name": {"#svc@h"}, "config": {`{"secret":"` + refusedSecret},
	}, 400)
	if !strings.Contains(refusedConfig, "submitted configuration is not shown again") ||
		strings.Contains(refusedConfig, refusedSecret) ||
		!strings.Contains(refusedConfig, `name=config rows=6 cols=60 required autocomplete=off`) {
		t.Fatal("refused private configuration was not cleared safely")
	}
	page = request("admin@h", "GET", "/service?name=%23svc@h", "", nil, 200)
	if !strings.Contains(page, "Danger Zone") || strings.Contains(page, "Replace configuration") {
		t.Fatal("daemon owner detail did not use the same Danger Zone boundary")
	}
	danger = request("admin@h", "GET", "/service-danger?name=%23svc@h", "", nil, 200)
	for _, label := range []string{"Replace configuration", "Transfer ownership", "Remove registration"} {
		if !strings.Contains(danger, label) {
			t.Fatalf("daemon owner Danger Zone missing node-wide control: %s", label)
		}
	}
	for _, who := range []string{"owner@h", "admin@h"} {
		request(who, "POST", "/service-confirm", "https://evil.example", url.Values{"action": {"delete"}, "name": {"#svc@h"}}, 403)
		request(who, "POST", "/service-confirm", "", url.Values{"action": {"delete"}, "name": {"#svc@h"}}, 403)
	}
	request("owner@h", "POST", "/service-confirm", "https://evil.example", url.Values{"action": {"transfer"}, "name": {"#svc@h"}, "owner": {"other@h"}}, 403)
	request("owner@h", "POST", "/service-confirm", "", url.Values{"action": {"transfer"}, "name": {"#svc@h"}, "owner": {"other@h"}}, 403)
	confirm := request("owner@h", "POST", "/service-confirm", web.URL, url.Values{"action": {"transfer"}, "name": {"#svc@h"}, "owner": {"other@h"}}, 200)
	if !strings.Contains(confirm, "Confirm ownership transfer") || !strings.Contains(confirm, `name=confirmed value=1`) {
		t.Fatal("transfer did not stop on a server-rendered confirmation")
	}
	if record, _ := b.Lookup("owner@h", "#svc@h"); record.Owner != "owner@h" {
		t.Fatal("rendering the transfer confirmation changed the record")
	}
	hidden := request("other@h", "GET", "/service-danger?name=%23svc@h", "", nil, 404)
	missing := request("other@h", "GET", "/service-danger?name=%23missing@h", "", nil, 404)
	hiddenShape := hidden[strings.Index(hidden, "<main>"):]
	missingShape := missing[strings.Index(missing, "<main>"):]
	for _, spelling := range []string{"#svc@h", url.QueryEscape("#svc@h"), "%23svc@h"} {
		hiddenShape = strings.ReplaceAll(hiddenShape, spelling, "NAME")
	}
	for _, spelling := range []string{"#missing@h", url.QueryEscape("#missing@h"), "%23missing@h"} {
		missingShape = strings.ReplaceAll(missingShape, spelling, "NAME")
	}
	if !strings.Contains(hidden, "No such name") || hiddenShape != missingShape {
		t.Fatal("Danger Zone distinguished a hidden record from a missing one")
	}
	if body := request("owner@h", "GET", "/agents?scope=my&state=active", "", nil, 200); !strings.Contains(body, "#svc@h") {
		t.Fatal("own active agent missing")
	}
	disable := url.Values{"action": {"deactivate"}, "name": {"#svc@h"}}
	request("owner@h", "POST", "/service", "https://evil.example", disable, 403)
	request("owner@h", "POST", "/service", "", disable, 403)
	request("other@h", "POST", "/service", web.URL, disable, 403)
	request("admin@h", "POST", "/service", web.URL, disable, 303)
	request("owner@h", "POST", "/service", web.URL, disable, 303)
	if body := request("owner@h", "GET", "/agents?scope=my&state=active", "", nil, 200); strings.Contains(body, "#svc@h") {
		t.Fatal("inactive agent appears active")
	}
	if body := request("owner@h", "GET", "/agents?scope=my&state=inactive", "", nil, 200); !strings.Contains(body, "#svc@h") {
		t.Fatal("inactive agent missing")
	}
	// Inactive is no such record to every edit but its reactivation.
	request("owner@h", "POST", "/service", web.URL, url.Values{"action": {"save"}, "name": {"#svc@h"}, "descr": {"while inactive"}}, 404)
	request("owner@h", "POST", "/service", web.URL, url.Values{"action": {"reactivate"}, "name": {"#svc@h"}}, 303)
	if body := request("owner@h", "GET", "/agents?scope=my&state=active", "", nil, 200); !strings.Contains(body, "#svc@h") {
		t.Fatal("reactivated agent is not active")
	}
	// A Group is an ordinary record, so an ordinary user may create one; an
	// existing group is its owner's and the Administrators', and nobody else's.
	group := url.Values{"action": {"save"}, "name": {"@ops"}, "members": {"other@h\nadmin@h"}}
	request("admin@h", "POST", "/groups", web.URL, group, 303)
	request("owner@h", "POST", "/groups", web.URL, url.Values{"action": {"save"}, "name": {"@ops"}, "members": {"owner@h"}}, 403)
	groups := request("admin@h", "GET", "/groups", "", nil, 200)
	if !strings.Contains(groups, `<table class="record-table group-table">`) || !strings.Contains(groups, `href="/group?name=%40ops"`) || !strings.Contains(groups, `<code>admin@h</code>, <code>other@h</code>`) || strings.Contains(groups, `<textarea name=members`) {
		t.Fatalf("group listing did not become a linked membership table: %s", groups)
	}
	groupDetail := request("admin@h", "GET", "/group?name=%40ops", "", nil, 200)
	if strings.Contains(groupDetail, `<textarea name=members`) || !strings.Contains(groupDetail, `id=members-edit class=editor-link`) {
		t.Fatalf("the group page carries an editor instead of linking to it: %s", groupDetail)
	}
	groupEditor := request("admin@h", "GET", "/group/edit?name=%40ops", "", nil, 200)
	if !strings.Contains(groupEditor, `<textarea name=members rows=8`) || !strings.Contains(groupEditor, `>admin@h
other@h</textarea>`) || strings.Contains(groupEditor, `<input name=members`) {
		t.Fatalf("group membership did not round-trip through its line editor: %s", groupEditor)
	}
	ordinaryGroup := request("other@h", "GET", "/group?name=%40ops", "", nil, 200)
	if strings.Contains(ordinaryGroup, `<textarea name=members`) || !strings.Contains(ordinaryGroup, `Membership is not visible to you.`) {
		t.Fatalf("ordinary user either gained the group editor or lost visible membership: %s", ordinaryGroup)
	}
	ordinaryGroups := request("other@h", "GET", "/groups", "", nil, 200)
	if !strings.Contains(ordinaryGroups, `Not visible to you`) || strings.Contains(ordinaryGroups, `No members`) {
		t.Fatalf("ordinary group list represented hidden membership as empty: %s", ordinaryGroups)
	}
	operatorGroup := request("operator@h", "GET", "/group/edit?name=%40ops", "", nil, 200)
	if !strings.Contains(operatorGroup, `<textarea name=members rows=8`) {
		t.Fatal("Administrator lost ordinary-group editing")
	}
	protectedGroup := request("operator@h", "GET", "/group?name=%40administrators", "", nil, 200)
	if strings.Contains(protectedGroup, `id=members-edit`) || !strings.Contains(protectedGroup, `Only the daemon owner changes this protected group.`) || !strings.Contains(protectedGroup, `popovertarget=administrators-help`) || !strings.Contains(protectedGroup, `Administering the node is not managing its resources.`) {
		t.Fatal("Administrator was offered the protected-group editor")
	}
	// And the form itself refuses, rather than only the link being absent.
	request("operator@h", "GET", "/group/edit?name=%40administrators", "", nil, 403)
	request("admin@h", "GET", "/group?name=%40missing", "", nil, 404)
	refusedGroup := request("admin@h", "POST", "/groups", web.URL, url.Values{
		"action": {"save"}, "name": {"@administrators"}, "members": {"admin@h\n@ops"},
	}, 400)
	if !strings.Contains(refusedGroup, "Check this form") ||
		!strings.Contains(refusedGroup, ">admin@h\n@ops</textarea>") ||
		!strings.Contains(refusedGroup, `aria-invalid="true"`) {
		t.Fatal("refused group edit did not preserve its line list and error state")
	}
	// A group is retired by emptying it, so "delete" is not an action here
	// even for the daemon owner, and even from a request that is otherwise
	// entirely in order — right origin, right session, real group
	// (docs/01-identity-and-roles.md#groups).
	retiredDelete := request("admin@h", "POST", "/groups", web.URL, url.Values{"action": {"delete"}, "name": {"@ops"}}, 400)
	if !strings.Contains(retiredDelete, "That request was not understood") ||
		!strings.Contains(retiredDelete, "Nothing was sent to the daemon") ||
		!strings.Contains(retiredDelete, `<a class=skip-link href=#main>`) {
		t.Fatal("retired group action did not use scoped browser recovery")
	}
	// And the members it carried were not applied on the way out: a rejected
	// action does nothing, rather than doing the save it was not asked for.
	request("admin@h", "POST", "/groups", web.URL, url.Values{"action": {"delete"}, "name": {"@ops"}, "members": {"admin@h"}}, 400)
	// Classification and sharing have no form of their own any more: they are
	// fields of Edit settings, and a save states separately that it carried
	// them so that a caller who may not edit them cannot clear them.
	request("owner@h", "POST", "/service", web.URL, url.Values{"action": {"save"}, "name": {"#svc@h"}, "edit_sharing": {"1"}, "maintainers": {"@ops\nadmin@h"}}, 303)
	page = request("owner@h", "GET", "/agent/edit?name=%23svc@h", "", nil, 200)
	if !strings.Contains(page, `<textarea name=maintainers rows=5`) || !strings.Contains(page, `>@ops
admin@h</textarea>`) {
		t.Fatalf("Maintainers list did not round-trip through its line editor: %s", page)
	}
	refusedMaintainers := request("owner@h", "POST", "/service", web.URL, url.Values{
		"action": {"save"}, "name": {"#svc@h"}, "edit_sharing": {"1"},
		"edit_allow": {"1"}, "allow": {"owner@h"}, "maintainers": {"missing@h\n@ops"},
	}, 404)
	if !strings.Contains(refusedMaintainers, "Check this form") ||
		!strings.Contains(refusedMaintainers, `href="#form-save"`) ||
		!strings.Contains(refusedMaintainers, ">missing@h\n@ops</textarea>") ||
		!strings.Contains(refusedMaintainers, ">owner@h</textarea>") {
		t.Fatalf("a refused sharing edit did not recover into Edit settings with its lines intact: %s", refusedMaintainers)
	}
	page = request("other@h", "GET", "/agent/edit?name=%23svc@h", "", nil, 200)
	if !strings.Contains(page, "Save settings") || !strings.Contains(page, "Danger Zone") || strings.Contains(page, "Transfer ownership") {
		t.Fatal("maintainer controls wrong")
	}
	danger = request("other@h", "GET", "/service-danger?name=%23svc@h", "", nil, 200)
	if !strings.Contains(danger, "Replace configuration") || !strings.Contains(danger, "Remove registration") || strings.Contains(danger, "Transfer ownership") {
		t.Fatal("maintainer Danger Zone controls wrong")
	}
	// A Maintainer may take the record out of service and bring it back.
	request("other@h", "POST", "/service", web.URL, disable, 303)
	if _, ok := b.Lookup("other@h", "#svc@h"); ok {
		t.Fatal("a Maintainer's deactivation left the record readable")
	}
	request("other@h", "POST", "/service", web.URL, url.Values{"action": {"reactivate"}, "name": {"#svc@h"}}, 303)
	if _, ok := b.Lookup("other@h", "#svc@h"); !ok {
		t.Fatal("a Maintainer's reactivation did not bring the record back")
	}
	request("other@h", "POST", "/service", web.URL, url.Values{"action": {"transfer"}, "name": {"#svc@h"}, "owner": {"other@h"}}, 403)
	request("owner@h", "POST", "/service", web.URL, url.Values{"action": {"save"}, "name": {"#svc@h"}, "descr": {"updated"}, "allow": {"owner@h"}, "bound": {"3"}, "overflow": {"strict"}}, 303)
	record, _ := b.Lookup("owner@h", "#svc@h")
	if record.Descr != "updated" || record.Bound != 3 {
		t.Fatal("form did not change daemon record")
	}
	request("owner@h", "POST", "/service", web.URL, url.Values{"action": {"configure"}, "name": {"#svc@h"}, "config": {`{"secret":"NEVER-RENDER-THIS","new":true}`}}, 303)
	request("owner@h", "POST", "/service", web.URL, url.Values{"action": {"transfer"}, "name": {"#svc@h"}, "owner": {"other@h"}}, 303)
	request("owner@h", "POST", "/service", web.URL, disable, 403)
	request("other@h", "POST", "/service", web.URL, url.Values{"action": {"delete"}, "name": {"#svc@h"}}, 303)
	if _, ok := b.Lookup("other@h", "#svc@h"); ok {
		t.Fatal("delete form did not unregister")
	}
}

func TestSameOrigin(t *testing.T) {
	for _, test := range []struct {
		origin, site string
		want         bool
	}{
		{"https://bus.local:8443", "same-origin", true},
		{"http://bus.local:8443", "same-origin", false},
		{"https://bus.local", "same-origin", false},
		{"https://bus.local:8443", "cross-site", false},
		{"https://bus.local:8443/evil", "", false},
		{"https://attacker@bus.local:8443", "", false},
		{"null", "", false},
	} {
		r := httptest.NewRequest("POST", "https://bus.local:8443/service", nil)
		r.Header.Set("Origin", test.origin)
		r.Header.Set("Sec-Fetch-Site", test.site)
		if sameOrigin(r, true) != test.want {
			t.Errorf("origin %s site %s", test.origin, test.site)
		}
	}
}
