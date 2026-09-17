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
	b.Masters([]string{"admin@h"})
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
	for _, record := range []protocol.Record{
		{Name: "peer-service@h", Owner: "peer@h"},
		{Name: "alice-normal@h", Owner: "alice@h", Allow: []string{"peer-service@h"}},
		{Name: "alice-personal@h", Owner: "alice@h", Allow: []string{"peer-service@h"}, Personal: true},
		{Name: "bob-personal@h", Owner: "bob@h", Allow: []string{"peer-service@h"}, Personal: true},
		{Name: "jobs@h", Owner: "alice@h", Kind: protocol.KindTopic, Mode: "queue"},
	} {
		if _, err := p.bus.Register(record); err != nil {
			t.Fatal(err)
		}
	}

	services, _ := p.request("alice@h", "GET", "/services?scope=my", nil, 200)
	if !strings.Contains(services, "alice-normal@h") || strings.Contains(services, "alice-personal@h") {
		t.Fatalf("main services did not exclude even the visitor's own Personal service: %s", services)
	}
	personal, _ := p.request("alice@h", "GET", "/personal", nil, 200)
	if !strings.Contains(personal, "alice-personal@h") || strings.Contains(personal, "alice-normal@h") || strings.Contains(personal, "bob-personal@h") {
		t.Fatalf("ordinary Personal page is not the owner's Personal-only view: %s", personal)
	}
	if !strings.Contains(personal, `<a href=/personal aria-current=page>`) || strings.Count(personal, "aria-current=page>") != 1 {
		t.Fatal("Personal navigation is not the one current entry")
	}
	_, header := p.request("alice@h", "GET", "/personal?owner=bob%40h", nil, http.StatusSeeOther)
	if header.Get("Location") != "/personal" {
		t.Fatalf("ordinary owner filter was silently reinterpreted: %q", header.Get("Location"))
	}
	channels, _ := p.request("alice@h", "GET", "/channels", nil, 200)
	if !strings.Contains(channels, "jobs@h") || !strings.Contains(channels, `<a href=/channels aria-current=page>`) {
		t.Fatal("Personal grouping changed channels or their current navigation")
	}
	detail, _ := p.request("alice@h", "GET", "/service?name=alice-personal%40h", nil, 200)
	if !strings.Contains(detail, "alice-personal@h") || !strings.Contains(detail, "· Personal") || !strings.Contains(detail, `<a href=/personal aria-current=page>`) {
		t.Fatal("direct Personal detail is not reachable and labelled")
	}

	admin, _ := p.request("admin@h", "GET", "/personal?owner=alice%40h", nil, 200)
	if !strings.Contains(admin, "alice-personal@h") || strings.Contains(admin, "bob-personal@h") || !strings.Contains(admin, "only Personal services visible through your normal access") {
		t.Fatalf("daemon-owner visible-only owner view is misstated: %s", admin)
	}
}

func TestPersonalOwnerEditsClassificationAndSharingAtomically(t *testing.T) {
	p := personalWebFixture(t)
	if err := p.bus.SetGroup("admin@h", "@ops", []string{"bob@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.bus.Register(protocol.Record{Name: "peer-service@h", Owner: "peer@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.bus.Register(protocol.Record{Name: "toggle@h", Owner: "alice@h", Allow: []string{"peer-service@h"}}); err != nil {
		t.Fatal(err)
	}

	enable := url.Values{"action": {"personal"}, "name": {"toggle@h"}, "personal": {"on"}, "allow": {"peer-service@h"}}
	_, header := p.request("alice@h", "POST", "/service", enable, http.StatusSeeOther)
	if header.Get("Location") != "/service?name=toggle%40h" {
		t.Fatalf("Personal update returned to %q", header.Get("Location"))
	}
	record, ok := p.bus.Lookup("alice@h", "toggle@h")
	if !ok || !record.Personal || len(record.Maintainers) != 0 || strings.Join(record.Allow, " ") != "peer-service@h" {
		t.Fatalf("atomic Personal enable: %+v", record)
	}
	page, _ := p.request("alice@h", "GET", "/service?name=toggle%40h", nil, 200)
	if strings.Count(page, `name=allow`) != 1 || !strings.Contains(page, "Classification and sharing") || !strings.Contains(page, "Personal classification, Allow and Maintainers are changed together") {
		t.Fatal("Personal detail exposes two Allow edit doors")
	}
	settings := url.Values{"action": {"save"}, "name": {"toggle@h"}, "descr": {"kept sharing"}, "bound": {"0"}, "overflow": {"strict"}}
	p.request("alice@h", "POST", "/service", settings, http.StatusSeeOther)
	record, _ = p.bus.Lookup("alice@h", "toggle@h")
	if strings.Join(record.Allow, " ") != "peer-service@h" || !record.Personal {
		t.Fatalf("ordinary settings silently wiped Personal sharing: %+v", record)
	}

	disable := url.Values{"action": {"personal"}, "name": {"toggle@h"}, "allow": {"bob@h"}, "maintainers": {"@ops"}}
	p.request("alice@h", "POST", "/service", disable, http.StatusSeeOther)
	record, ok = p.bus.Lookup("alice@h", "toggle@h")
	if !ok || record.Personal || strings.Join(record.Maintainers, " ") != "@ops" || strings.Join(record.Allow, " ") != "bob@h" {
		t.Fatalf("atomic Personal disable and sharing: %+v", record)
	}
	maintainer, _ := p.request("bob@h", "GET", "/service?name=toggle%40h", nil, 200)
	if strings.Contains(maintainer, "Classification and sharing") {
		t.Fatal("a Maintainer received the owner-only classification control")
	}
}

func TestPersonalCreationUsesCoreValidation(t *testing.T) {
	p := personalWebFixture(t)
	if _, err := p.bus.Register(protocol.Record{Name: "peer-service@h", Owner: "peer@h"}); err != nil {
		t.Fatal(err)
	}
	create := url.Values{"action": {"create"}, "name": {"new-personal@h"}, "kind": {"generic"}, "personal": {"on"}, "allow": {"peer-service@h"}}
	_, header := p.request("alice@h", "POST", "/service", create, http.StatusSeeOther)
	if header.Get("Location") != "/service?name=new-personal%40h" {
		t.Fatalf("Personal creation returned to %q", header.Get("Location"))
	}
	if got, ok := p.bus.Lookup("alice@h", "new-personal@h"); !ok || !got.Personal {
		t.Fatalf("web creation lost Personal: %+v, %v", got, ok)
	}
	bad := url.Values{"action": {"create"}, "name": {"bad-personal@h"}, "kind": {"generic"}, "personal": {"on"}, "allow": {"bob@h"}}
	p.request("alice@h", "POST", "/service", bad, http.StatusBadRequest)
	if _, ok := p.bus.Lookup("alice@h", "bad-personal@h"); ok {
		t.Fatal("web bypassed core Personal validation")
	}
}
