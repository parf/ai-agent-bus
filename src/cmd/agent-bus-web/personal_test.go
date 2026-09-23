package main

import (
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

type personalWeb struct {
	t        *testing.T
	bus      *core.Bus
	web      *httptest.Server
	client   *http.Client
	sessions map[string]*http.Cookie
}

func personalWebFixture(t *testing.T) *personalWeb {
	t.Helper()
	b := core.New()
	known(t, b, "alice@h", "bob@h", "peer@h")
	tokens, err := auth.Load(memory.NewTokens(), "admin@h")
	if err != nil {
		t.Fatal(err)
	}
	backend := httptest.NewServer(api.New(b, tokens, "admin@h").Handler())
	t.Cleanup(backend.Close)
	web := httptest.NewServer(dashboard(&caller{client: backend.Client(), base: backend.URL}, false))
	t.Cleanup(web.Close)
	client := web.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	p := &personalWeb{t: t, bus: b, web: web, client: client, sessions: map[string]*http.Cookie{}}
	for _, who := range []string{"alice@h", "bob@h", "admin@h"} {
		token, err := tokens.Issue(who)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.PostForm(web.URL+"/signin", url.Values{"token": {token}})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther || len(resp.Cookies()) != 1 {
			t.Fatalf("sign in %s: %d", who, resp.StatusCode)
		}
		p.sessions[who] = resp.Cookies()[0]
	}
	return p
}

func (p *personalWeb) request(who, method, path string, form url.Values, want int) (string, http.Header) {
	p.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, p.web.URL+path, body)
	if err != nil {
		p.t.Fatal(err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", p.web.URL)
	}
	req.AddCookie(p.sessions[who])
	resp, err := p.client.Do(req)
	if err != nil {
		p.t.Fatal(err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		p.t.Fatalf("%s %s %s: %d %s, want %d", who, method, path, resp.StatusCode, payload, want)
	}
	return string(payload), resp.Header
}

func TestPersonalServicesAreGroupedWithoutChangingAccess(t *testing.T) {
	p := personalWebFixture(t)
	// A Personal record may name only its owner's cohort, so each one shares
	// with its owner's own agents or @owner and with nobody else.
	for _, record := range []protocol.Record{
		{Kind: protocol.KindAgent, Name: "#peer-service@h", Owner: "peer@h"},
		{Kind: protocol.KindAgent, Name: "#alice-normal@h", Owner: "alice@h", Allow: []string{"#peer-service@h"}},
		{Kind: protocol.KindAgent, Name: "#alice-personal@h", Owner: "alice@h", Allow: []string{"#alice-normal@h"}, Personal: true},
		{Kind: protocol.KindAgent, Name: "#bob-personal@h", Owner: "bob@h", Allow: []string{"@owner"}, Personal: true},
		{Name: "jobs@h", Owner: "alice@h", Kind: protocol.KindQueue},
		// Personal is valid on every kind, and the Personal tab lists them all.
		{Name: "alice-inbox@h", Owner: "alice@h", Kind: protocol.KindQueue, Allow: []string{"@owner"}, Personal: true},
	} {
		if _, err := p.bus.Register(record); err != nil {
			t.Fatal(err)
		}
	}

	services, _ := p.request("alice@h", "GET", "/agents?scope=my", nil, 200)
	if !strings.Contains(services, "#alice-normal@h") || strings.Contains(services, "#alice-personal@h") {
		t.Fatalf("the agents page did not exclude even the visitor's own Personal agent: %s", services)
	}
	personal, _ := p.request("alice@h", "GET", "/personal", nil, 200)
	if !strings.Contains(personal, "#alice-personal@h") || strings.Contains(personal, "#alice-normal@h") || strings.Contains(personal, "#bob-personal@h") {
		t.Fatalf("ordinary Personal page is not the owner's Personal-only view: %s", personal)
	}
	// A Personal queue is listed there too; the owner's own user record, which
	// is always Personal, is not. Asked of the row link: the signed-in name is
	// in the header of every page.
	if !strings.Contains(personal, `class=record-name href="/queue?name=alice-inbox%40h`) || strings.Contains(personal, `class=record-name href="/queue?name=alice%40h`) {
		t.Fatalf("the Personal page does not list Personal records of every kind but users: %s", personal)
	}
	if !strings.Contains(personal, `<a href=/agents aria-current=page>`) || strings.Count(personal, "aria-current=page>") != 1 {
		t.Fatal("Agents navigation is not the one current entry for its Personal subview")
	}
	if !strings.Contains(personal, `class="record-name-cell owned-record personal-record"`) || strings.Contains(personal, "Yours") || !strings.Contains(personal, `class=personal-marker>Personal</span>`) {
		t.Fatal("caller-owned Personal row does not keep both visible distinctions")
	}
	_, header := p.request("alice@h", "GET", "/personal?owner=bob%40h", nil, http.StatusSeeOther)
	if header.Get("Location") != "/personal" {
		t.Fatalf("ordinary owner filter was silently reinterpreted: %q", header.Get("Location"))
	}
	channels, _ := p.request("alice@h", "GET", "/queues", nil, 200)
	if !strings.Contains(channels, `class=record-name href="/queue?name=jobs%40h`) || strings.Contains(channels, "alice-inbox@h") || !strings.Contains(channels, `<a href=/queues aria-current=page>`) {
		t.Fatal("Personal grouping changed channels or their current navigation")
	}
	detail, _ := p.request("alice@h", "GET", "/agent?name=%23alice-personal%40h", nil, 200)
	if !strings.Contains(detail, "#alice-personal@h") || !strings.Contains(detail, "· Personal") || !strings.Contains(detail, `<a href=/agents aria-current=page>`) {
		t.Fatal("direct Personal detail is not reachable and labelled")
	}

	// A Back link names the listing the record is on. A return pointing at any
	// other one, however local it is, would send the visitor to a page that
	// cannot show what they just left.
	borrowed, _ := p.request("alice@h", "GET", "/agent?name=%23alice-personal%40h&return=%2fservices", nil, 200)
	if !strings.Contains(borrowed, `<p><a href="/personal">Back to records</a></p>`) {
		t.Fatal("a Personal agent offered a Back link to a listing it is not on")
	}
	kept, _ := p.request("alice@h", "GET", "/agent?name=%23alice-normal%40h&return=%2fagents%3fpage%3d1", nil, 200)
	if !strings.Contains(kept, `<p><a href="/agents?page=1">Back to records</a></p>`) {
		t.Fatal("the agents list state was lost on the way to a record and back")
	}
	admin, _ := p.request("admin@h", "GET", "/personal?owner=alice%40h", nil, 200)
	if !strings.Contains(admin, "#alice-personal@h") || strings.Contains(admin, "#bob-personal@h") || !strings.Contains(admin, "only Personal records visible through your normal access") {
		t.Fatalf("daemon-owner visible-only owner view is misstated: %s", admin)
	}
	if !strings.Contains(admin, `class="record-name-cell personal-record"`) || strings.Contains(admin, `class="record-name-cell owned-record personal-record"`) || strings.Contains(admin, "Yours") || !strings.Contains(admin, `class=personal-marker>Personal</span>`) {
		t.Fatal("a visible Personal row owned by somebody else was confused with Yours")
	}
	// Removing a Personal agent returns to the Personal view, not to Agents,
	// which excludes it.
	_, header = p.request("alice@h", "POST", "/service", url.Values{
		"action": {"delete"}, "name": {"#alice-personal@h"}, "confirmed": {"1"},
		"expected_owner": {"alice@h"}, "expected_queued": {"0"}, "expected_readers": {"0"},
	}, http.StatusSeeOther)
	if header.Get("Location") != "/personal" {
		t.Fatalf("a removed Personal agent returned to %q", header.Get("Location"))
	}
}

func TestPersonalOwnerEditsClassificationAndSharingAtomically(t *testing.T) {
	p := personalWebFixture(t)
	if err := p.bus.SetGroup("admin@h", "@ops", []string{"bob@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.bus.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#peer-service@h", Owner: "peer@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.bus.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#toggle@h", Owner: "alice@h", Allow: []string{"#peer-service@h"}}); err != nil {
		t.Fatal(err)
	}
	// alice@h's own agent, which a Personal record of hers may name.
	if _, err := p.bus.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#helper@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}

	// Classification, Allow and Maintainers are fields of Edit settings now.
	// The two flags are what the form states about itself: that it carried
	// them, and is therefore allowed to change them.
	enable := url.Values{"action": {"save"}, "name": {"#toggle@h"}, "edit_sharing": {"1"}, "edit_personal": {"1"}, "personal": {"on"}, "edit_allow": {"1"}, "allow": {"#helper@h"}}
	_, header := p.request("alice@h", "POST", "/service", enable, http.StatusSeeOther)
	if header.Get("Location") != "/agent?name=%23toggle%40h" {
		t.Fatalf("Personal update returned to %q", header.Get("Location"))
	}
	record, ok := p.bus.Lookup("alice@h", "#toggle@h")
	if !ok || !record.Personal || len(record.Maintainers) != 0 || strings.Join(record.Allow, " ") != "#helper@h" {
		t.Fatalf("atomic Personal enable: %+v", record)
	}
	// Every save of a Personal record is checked: another user's agent is
	// outside alice@h's cohort, so the face passes on the daemon's refusal.
	outside := url.Values{"action": {"save"}, "name": {"#toggle@h"}, "edit_sharing": {"1"}, "edit_personal": {"1"}, "personal": {"on"}, "edit_allow": {"1"}, "allow": {"#peer-service@h"}}
	if refused, _ := p.request("alice@h", "POST", "/service", outside, http.StatusBadRequest); !strings.Contains(refused, "reaches outside the owner") {
		t.Errorf("a Personal save was refused for some other reason: %s", refused)
	}
	if record, _ := p.bus.Lookup("alice@h", "#toggle@h"); strings.Join(record.Allow, " ") != "#helper@h" {
		t.Fatalf("a Personal save reached outside its owner's cohort: %+v", record)
	}
	page, _ := p.request("alice@h", "GET", "/agent/edit?name=%23toggle%40h", nil, 200)
	if strings.Count(page, `name=allow`) != 1 || strings.Contains(page, "Classification and sharing") {
		t.Fatalf("Personal detail does not have one editor with one Allow door: %s", page)
	}
	if !strings.Contains(page, `<input type=hidden name=edit_sharing value=1>`) || !strings.Contains(page, `<input type=hidden name=edit_personal value=1>`) {
		t.Fatalf("the owner's settings form does not say it carries classification and sharing: %s", page)
	}
	// A save that did not carry them leaves them alone. This is the whole
	// reason the form states it: a Maintainer's save posts neither flag.
	settings := url.Values{"action": {"save"}, "name": {"#toggle@h"}, "descr": {"kept sharing"}, "bound": {"0"}, "overflow": {"strict"}}
	p.request("alice@h", "POST", "/service", settings, http.StatusSeeOther)
	record, _ = p.bus.Lookup("alice@h", "#toggle@h")
	if strings.Join(record.Allow, " ") != "#helper@h" || !record.Personal {
		t.Fatalf("ordinary settings silently wiped Personal sharing: %+v", record)
	}

	disable := url.Values{"action": {"save"}, "name": {"#toggle@h"}, "edit_sharing": {"1"}, "edit_personal": {"1"}, "edit_allow": {"1"}, "allow": {"bob@h"}, "maintainers": {"@ops"}}
	p.request("alice@h", "POST", "/service", disable, http.StatusSeeOther)
	record, ok = p.bus.Lookup("alice@h", "#toggle@h")
	if !ok || record.Personal || strings.Join(record.Maintainers, " ") != "@ops" || strings.Join(record.Allow, " ") != "bob@h" {
		t.Fatalf("atomic Personal disable and sharing: %+v", record)
	}
	// A Maintainer sees the same one editor, with the two owner-only fields
	// shown and disabled, and without the flags that would let a save carry
	// them. Both halves are asserted: a page that dropped the fields would
	// pass a check for the missing flags alone.
	maintainer, _ := p.request("bob@h", "GET", "/agent/edit?name=%23toggle%40h", nil, 200)
	if strings.Contains(maintainer, "edit_sharing") || strings.Contains(maintainer, "edit_personal") {
		t.Fatalf("a Maintainer's form claims it may change classification and sharing: %s", maintainer)
	}
	if !strings.Contains(maintainer, `<textarea name=maintainers rows=5 disabled`) ||
		!strings.Contains(maintainer, `<input type=checkbox name=personal disabled`) {
		t.Fatalf("a Maintainer cannot see the owner-only fields at all: %s", maintainer)
	}
	// And posting them by hand changes nothing, because the daemon decides.
	p.request("bob@h", "POST", "/service", url.Values{
		"action": {"save"}, "name": {"#toggle@h"}, "edit_sharing": {"1"}, "maintainers": {""},
	}, http.StatusForbidden)
	record, _ = p.bus.Lookup("alice@h", "#toggle@h")
	if strings.Join(record.Maintainers, " ") != "@ops" {
		t.Fatalf("a Maintainer cleared the Maintainers list: %+v", record)
	}
}

func TestPersonalCreationUsesCoreValidation(t *testing.T) {
	p := personalWebFixture(t)
	if _, err := p.bus.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#peer-service@h", Owner: "peer@h"}); err != nil {
		t.Fatal(err)
	}
	create := url.Values{"action": {"create"}, "name": {"#new-personal@h"}, "kind": {protocol.KindAgent}, "personal": {"on"}, "allow": {"@owner"}}
	_, header := p.request("alice@h", "POST", "/service", create, http.StatusSeeOther)
	if header.Get("Location") != "/agent?name=%23new-personal%40h" {
		t.Fatalf("Personal creation returned to %q", header.Get("Location"))
	}
	if got, ok := p.bus.Lookup("alice@h", "#new-personal@h"); !ok || !got.Personal || got.Owner != "alice@h" {
		t.Fatalf("web creation lost Personal: %+v, %v", got, ok)
	}
	// A Personal record shares with its owner's cohort only, so the wildcard
	// and another user's agent are the refusals the face must not talk the
	// daemon out of.
	for _, allow := range []string{"*", "#peer-service@h"} {
		bad := url.Values{"action": {"create"}, "name": {"#bad-personal@h"}, "kind": {protocol.KindAgent}, "personal": {"on"}, "allow": {allow}}
		refused, _ := p.request("alice@h", "POST", "/service", bad, http.StatusBadRequest)
		if !strings.Contains(refused, "reaches outside the owner") {
			t.Errorf("%s was refused for some other reason: %s", allow, refused)
		}
		if _, ok := p.bus.Lookup("alice@h", "#bad-personal@h"); ok {
			t.Fatalf("web bypassed core Personal validation for %s", allow)
		}
	}
}
