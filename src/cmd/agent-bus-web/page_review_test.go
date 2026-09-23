package main

import (
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

// reviewFixture is personalWebFixture with the daemon owner a User too, so
// its own user page and credential row exist.
func reviewFixture(t *testing.T) *personalWeb {
	t.Helper()
	b := core.New()
	known(t, b, "admin@h", "bob@h")
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
	for _, who := range []string{"admin@h", "bob@h"} {
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

// A shared list that is empty only because its records are Personal says so,
// and where they are, rather than claiming there are none. The rule that All
// and My omit Personal records is unchanged.
func TestEmptySharedListPointsToItsPersonalRecords(t *testing.T) {
	p := reviewFixture(t)
	for _, name := range []string{"#bob-one@h", "#bob-two@h"} {
		if _, err := p.bus.Register(protocol.Record{Kind: protocol.KindAgent, Name: name, Owner: "bob@h", Allow: []string{"@owner"}, Personal: true}); err != nil {
			t.Fatal(err)
		}
	}
	agents, _ := p.request("bob@h", "GET", "/agents", nil, 200)
	if strings.Contains(agents, "<h2>No agents yet</h2>") {
		t.Fatal("the Agents page claims no agents while two Personal ones exist")
	}
	if !strings.Contains(agents, `<section class="empty-state editor-card personal-elsewhere"><h2>No shared agents</h2>`) ||
		!strings.Contains(agents, `<p>2 Personal agents are under the <a class=personal-view href=/personal>Personal</a> tab, which this list omits.</p>`) {
		t.Fatalf("the empty Agents page does not point to its Personal agents: %s", agents)
	}
	if strings.Contains(agents, `class=record-name href="/agent?name=%23bob-one`) {
		t.Fatal("All began listing a Personal agent")
	}
	// Queues has no Personal queue, so its empty state is the plain one: the
	// Personal agents are no queue's to point at.
	queues, _ := p.request("bob@h", "GET", "/queues", nil, 200)
	if !strings.Contains(queues, `<section class="empty-state editor-card"><h2>No queues yet</h2>`) || strings.Contains(queues, "personal-elsewhere") {
		t.Fatalf("an empty Queues page pointed at Personal records of another kind: %s", queues)
	}
}

// The daemon owner's authority is stated once in the Identity card, not as
// both the identity and the authority pill.
func TestUserPageStatesTheDaemonOwnerOnce(t *testing.T) {
	p := reviewFixture(t)
	page, _ := p.request("admin@h", "GET", "/user?name=admin%40h", nil, 200)
	start := strings.Index(page, "<h2>Identity</h2>")
	end := strings.Index(page, "<h2>Groups</h2>")
	if start < 0 || end < start {
		t.Fatalf("no Identity card: %s", page)
	}
	card := page[start:end]
	if n := strings.Count(card, `<span class=fact-pill>🔱 Daemon owner</span>`); n != 1 {
		t.Fatalf("Identity card states the daemon owner %d times: %s", n, card)
	}
	bob, _ := p.request("admin@h", "GET", "/user?name=bob%40h", nil, 200)
	if !strings.Contains(bob, `<span class=fact-pill>👤 User</span><span class=fact-pill>User</span>`) {
		t.Fatal("an ordinary user's identity and authority pills changed")
	}
}

// The credentials table stacks on a phone as every record table does, and a
// User credential carries the User glyph beside its Agent neighbours.
func TestAccountCredentialsStackAndMarkTheUser(t *testing.T) {
	p := reviewFixture(t)
	page, _ := p.request("bob@h", "GET", "/account", nil, 200)
	if !strings.Contains(page, `<table class="record-table credentials-table">`) {
		t.Fatal("the credentials table is not a stacking record table")
	}
	if !strings.Contains(page, `<td data-label=Name><code>bob@h</code></td><td data-label=Kind>👤 User</td>`) {
		t.Fatalf("a User credential row is not labelled and marked: %s", page)
	}
}

// Diagnostics tables are small enough to fit a phone, so they take its width
// instead of shrinking into a scroll box of their own.
func TestDiagnosticsTablesFitAPhone(t *testing.T) {
	p := reviewFixture(t)
	page, _ := p.request("admin@h", "GET", "/diagnostics", nil, 200)
	for _, want := range []string{
		`table:not(.record-table):not(.fit-table){display:block;overflow-x:auto}`,
		`<table class=fit-table><caption>Refusals since this daemon started, by reason</caption><thead><tr><th scope=col>Reason<th scope=col class=num>Count</tr>`,
		`<table class=fit-table><caption>Inboxes holding messages`,
		`<table class=fit-table><thead><tr><th scope=col>Name<th scope=col class=num>Dropped`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("diagnostics lacks %q", want)
		}
	}
}

// Presentation rules the screenshots showed broken. They are CSS, so what a
// test can hold is that the served stylesheet carries each rule; the rendered
// result is the before and after screenshots.
func TestServedStylesheetCarriesTheLayoutFixes(t *testing.T) {
	p := reviewFixture(t)
	page, _ := p.request("admin@h", "GET", "/", nil, 200)
	for rule, why := range map[string]string{
		` .node-navigation nav{display:flex;flex-wrap:wrap;align-items:center;gap:0 1.5rem;line-height:2.4}`: "the section navigation does not wrap on a phone",
		` select{max-width:100%}`:                                      "a long selector can push the page wider than a phone",
		` .page-title h1 code{font-size:1em}`:                          "a code-set page title shrinks to body code size",
		`.node-fact{display:flex;flex-direction:column;flex:1 1 7rem;`: "the eight node figures do not fit one row",
	} {
		if !strings.Contains(page, rule) {
			t.Errorf("%s: served stylesheet lacks %q", why, rule)
		}
	}
	if strings.Contains(page, "line-height:2.4;overflow-x:auto}") {
		t.Error("the section navigation still scrolls instead of wrapping")
	}
	groups, _ := p.request("admin@h", "GET", "/groups", nil, 200)
	if !strings.Contains(groups, `<a href="/groups" aria-current=true>All (`) {
		t.Error("the Groups tab is not named All, as every other section's is")
	}
}
