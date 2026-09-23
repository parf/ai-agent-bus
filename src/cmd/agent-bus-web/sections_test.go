package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/display"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// redirect asks for path without following the answer, and returns the status
// and where it points.
func (m *meanings) redirect(path string) (int, string) {
	m.t.Helper()
	req, err := http.NewRequest("GET", m.web.URL+path, nil)
	if err != nil {
		m.t.Fatal(err)
	}
	req.AddCookie(m.session)
	resp, err := m.client.Do(req)
	if err != nil {
		m.t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Location")
}

// Queues and PubSub are two sections, each listing its own kind and nothing
// else, with no Kind switch left to tell them apart.
func TestQueuesAndPubSubAreSectionsOfTheirOwn(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "work@h", Owner: "admin@h", Kind: protocol.KindQueue, Allow: []string{"*"}})
	m.register(protocol.Record{Name: "shout@h", Owner: "admin@h", Kind: protocol.KindPubSub, Allow: []string{"*"}})
	queues, pubsub := m.get("/queues"), m.get("/pubsub")
	if !strings.Contains(queues, `href="/queue?name=work%40h`) || strings.Contains(queues, "shout@h") {
		t.Errorf("Queues does not list exactly the queue: %s", queues)
	}
	if !strings.Contains(pubsub, `href="/pubsub/topic?name=shout%40h`) || strings.Contains(pubsub, "work@h") {
		t.Errorf("PubSub does not list exactly the topic: %s", pubsub)
	}
	for name, page := range map[string]string{"Queues": queues, "PubSub": pubsub} {
		if strings.Contains(page, `aria-label="Kind filter"`) {
			t.Errorf("%s still offers a Kind switch", name)
		}
	}
	if !strings.Contains(queues, `<h1><span class=page-title-mark aria-hidden=true>📮</span> Queues</h1>`) {
		t.Errorf("Queues is not titled with its glyph: %s", section(t, queues, "<div class=page-title>", "</div>"))
	}
	if !strings.Contains(pubsub, `<h1><span class=page-title-mark aria-hidden=true>📣</span> PubSub</h1>`) {
		t.Errorf("PubSub is not titled with its glyph: %s", section(t, pubsub, "<div class=page-title>", "</div>"))
	}
	// Each counts its own kind: one of each is registered.
	if !strings.Contains(queues, `<a href="/queues" aria-current=true class="">All (1)</a>`) || !strings.Contains(pubsub, `<a href="/pubsub" aria-current=true class="">All (1)</a>`) {
		t.Errorf("a section counts the other kind: %s / %s", section(t, queues, `<nav class=section-nav`, "</nav>"), section(t, pubsub, `<nav class=section-nav`, "</nav>"))
	}
	if !strings.Contains(queues, `href="/queues/new" class="">Register queue</a>`) || !strings.Contains(pubsub, `href="/pubsub/new" class="">Register pub/sub topic</a>`) {
		t.Error("a section does not register its own kind")
	}
	// Each registration form is its own kind's, reached from its own section.
	if page := m.get("/queues/new"); !strings.Contains(page, `<input type=hidden name=kind value=queue>`) {
		t.Errorf("/queues/new is not the queue form: %s", page)
	}
	if page := m.get("/pubsub/new"); !strings.Contains(page, `<input type=hidden name=kind value=pubsub>`) {
		t.Errorf("/pubsub/new is not the pub/sub form: %s", page)
	}
	// Detail and settings pages are each kind's own address.
	if page := m.get("/pubsub/topic?name=shout%40h"); !strings.Contains(page, `href="/pubsub/topic/edit?name=shout%40h`) {
		t.Errorf("the topic page does not link its own settings: %s", page)
	}
	if page := m.get("/queue?name=work%40h"); !strings.Contains(page, `href="/queue/edit?name=work%40h`) {
		t.Errorf("the queue page does not link its own settings: %s", page)
	}
	m.get("/pubsub/topic/edit?name=shout%40h")
	m.get("/queue/edit?name=work%40h")
}

// A bookmark of the combined page lands on the section it meant.
func TestOldChannelAddressesStillLandSomewhere(t *testing.T) {
	m := meaningFixture(t)
	m.register(protocol.Record{Name: "work@h", Owner: "admin@h", Kind: protocol.KindQueue})
	for path, want := range map[string]string{
		"/channels":                                "/queues",
		"/channels?kind=queue&q=w":                 "/queues?q=w",
		"/channels?kind=pubsub&sort=updated":       "/pubsub?sort=updated",
		"/channels/new":                            "/queues/new",
		"/channels/new?kind=queue":                 "/queues/new",
		"/channels/new?kind=pubsub":                "/pubsub/new",
	} {
		code, to := m.redirect(path)
		if code != http.StatusMovedPermanently || (want != "" && to != want) {
			t.Errorf("%s: %d to %q, want 301 to %q", path, code, to, want)
		}
	}
	// The old detail and settings addresses still serve the record.
	if page := m.get("/channel?name=work%40h"); !strings.Contains(page, "work@h</h1>") {
		t.Errorf("/channel no longer serves the record: %s", page)
	}
	m.get("/channel/edit?name=work%40h")
}

// Every entity in the navigation is marked with its display glyph, and every
// one of them is marked: a glyph on one and not another is the menu
// disagreeing with itself.
func TestTheNavigationMarksEveryEntityWithItsGlyph(t *testing.T) {
	m := meaningFixture(t)
	page := m.get("/")
	nav := section(t, page, `<nav aria-label="sections">`, "</nav>")
	for _, e := range []struct{ href, kind, label string }{
		{"/agents", protocol.KindAgent, "Agents"},
		{"/services", protocol.KindService, "Services"},
		{"/queues", protocol.KindQueue, "Queues"},
		{"/pubsub", protocol.KindPubSub, "PubSub"},
		{"/users", protocol.KindUser, "Users"},
		{"/groups", protocol.KindGroup, "Groups"},
	} {
		want := `<a href=` + e.href + `><span class=page-title-mark aria-hidden=true>` + display.EntityGlyph(e.kind) + `</span>` + e.label + `</a>`
		if !strings.Contains(nav, want) {
			t.Errorf("the navigation lacks %s: %s", want, nav)
		}
	}
	order := []string{"/agents>", "/services>", "/queues>", "/pubsub>", "/users>", "/groups>"}
	at := -1
	for _, href := range order {
		i := strings.Index(nav, "href="+href)
		if i < at {
			t.Errorf("the navigation is out of order at %s: %s", href, nav)
		}
		at = i
	}
	if strings.Contains(nav, "Channels") {
		t.Errorf("the navigation still names Channels: %s", nav)
	}
	// Inside each section, its own entry is the current one.
	for path, href := range map[string]string{"/queues": "/queues", "/pubsub": "/pubsub", "/queues/new": "/queues", "/pubsub/new": "/pubsub"} {
		nav := section(t, m.get(path), `<nav aria-label="sections">`, "</nav>")
		if !strings.Contains(nav, `<a href=`+href+` aria-current=page>`) {
			t.Errorf("%s does not mark %s current: %s", path, href, nav)
		}
	}
}
