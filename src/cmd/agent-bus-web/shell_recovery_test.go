package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// A section that fails is not an answer of nothing, and it is not a place to
// print the transport error either: that text names the address this child
// talks to. Two sections rendered err.Error() straight into the page — the
// record page's activity and Account's credentials — and the check that
// covered the first looked only for the words "Activity unavailable", which
// the leak also satisfied. See Plans/MVP/done/web-overview-diagnostics.md for
// the same defect found on the envelope feed.
func TestADegradedSectionSaysSoWithoutNamingTheBackend(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "admin@h")
	if err != nil {
		t.Fatal(err)
	}
	face := api.New(b, tokens, "admin@h").Handler()
	// refused picks how the two section reads fail, and is read by the
	// handler goroutine while the test changes it, so it is atomic: false
	// drops the connection, which is the case whose error text carries the
	// address; true is a refusal the daemon worded, which stays caller-facing.
	var refused atomic.Bool
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/activity" || r.URL.Path == "/names" {
			if !refused.Load() {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				conn.Close()
				return
			}
			http.Error(w, `{"error":"history is not retained on this node"}`, http.StatusServiceUnavailable)
			return
		}
		face.ServeHTTP(w, r)
	}))
	defer backend.Close()
	web := httptest.NewServer(dashboard(&caller{client: backend.Client(), base: backend.URL}, false))
	defer web.Close()
	client := web.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	token, err := tokens.Issue("admin@h")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.PostForm(web.URL+"/signin", url.Values{"token": {token}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(resp.Cookies()) != 1 {
		t.Fatal("sign in did not return a session")
	}
	session := resp.Cookies()[0]
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "admin@h"}); err != nil {
		t.Fatal(err)
	}
	get := func(path string) (int, string) {
		t.Helper()
		req, err := http.NewRequest("GET", web.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(session)
		r, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(body)
	}
	// The address as the child holds it, and the bare host:port a template
	// would produce if any part of the transport text reached the page.
	where := strings.TrimPrefix(backend.URL, "http://")

	sections := map[string]string{"/service?name=%23svc@h": "Activity unavailable", "/account": "Credentials"}
	for path, marker := range sections {
		code, body := get(path)
		if code != http.StatusOK {
			t.Fatalf("%s answered %d with one section failing, and the rest of the page was true: %s", path, code, body)
		}
		if strings.Contains(body, where) || strings.Contains(body, backend.URL) {
			t.Errorf("%s puts the backend address on the page: %s", path, body)
		}
		if !strings.Contains(body, marker) {
			t.Errorf("%s no longer names the section that failed", path)
		}
		if !strings.Contains(body, "the daemon did not answer") {
			t.Errorf("%s does not say the section failed, so a reader cannot tell it from an answer of nothing: %s", path, body)
		}
	}
	// A refusal the daemon worded is the caller's to read, and it arrives
	// unwrapped from the JSON body rather than as the body itself.
	refused.Store(true)
	for path := range sections {
		_, body := get(path)
		if !strings.Contains(body, "history is not retained on this node") {
			t.Errorf("%s dropped the daemon's own wording for the refusal", path)
		}
		if strings.Contains(body, `{&#34;error&#34;`) || strings.Contains(body, `{"error"`) {
			t.Errorf("%s shows the raw JSON body rather than the message in it", path)
		}
	}
}

// The three recoveries the acceptance names must read differently, and the
// expired one must also read differently from never having signed in: both
// answer 401 from the same form, so "offers sign-in" cannot tell them apart.
func TestExpiredPermissionAndUnavailableRecoveriesAreFourDistinctAnswers(t *testing.T) {
	m := meaningFixture(t)
	signInAs := func(who string) *http.Cookie {
		t.Helper()
		if _, err := m.bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
		token, err := m.tokens.Issue(who)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := m.client.PostForm(m.web.URL+"/signin", url.Values{"token": {token}})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if len(resp.Cookies()) != 1 {
			t.Fatalf("%s could not sign in", who)
		}
		return resp.Cookies()[0]
	}
	ask := func(method, path string, c *http.Cookie, form url.Values) (int, string) {
		t.Helper()
		var input io.Reader
		if form != nil {
			input = strings.NewReader(form.Encode())
		}
		req, err := http.NewRequest(method, m.web.URL+path, input)
		if err != nil {
			t.Fatal(err)
		}
		if form != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", m.web.URL)
		}
		if c != nil {
			req.AddCookie(c)
		}
		resp, err := m.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	// Never signed in: the form, saying to sign in, and carrying the address
	// that was asked for.
	code, anonymous := ask("GET", "/services", nil, nil)
	if code != http.StatusUnauthorized || !strings.Contains(anonymous, "sign in to open this page") {
		t.Fatalf("an anonymous page answered %d without asking for a credential: %s", code, anonymous)
	}
	// A session the bus does not know is a session that ended, not a visitor
	// who never had one. Same form, different sentence, because the reader
	// did sign in and wants to know their session went, not that they
	// imagined it.
	code, ended := ask("GET", "/services", &http.Cookie{Name: cookieName, Value: "not-a-session"}, nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("a session the bus does not know answered %d, want 401: %s", code, ended)
	}
	if !strings.Contains(ended, "that session has ended") {
		t.Errorf("an ended session is not named as one: %s", ended)
	}
	if strings.Contains(ended, "sign in to open this page") {
		t.Error("an ended session reads as never having signed in")
	}
	if !strings.Contains(ended, "action=/signin") {
		t.Error("an ended session does not offer the one recovery that works")
	}
	// Refused for want of permission: signing in again cannot help, so it
	// must not be offered.
	plain := signInAs("plain@h")
	// A group somebody else owns: an ordinary user may create groups, but not
	// edit this one.
	if err := m.bus.SetGroup("admin@h", "@ops", []string{"admin@h"}); err != nil {
		t.Fatal(err)
	}
	code, refused := ask("POST", "/groups", plain, url.Values{"action": {"save"}, "name": {"@ops"}, "members": {"plain@h"}})
	if code != http.StatusForbidden || !strings.Contains(refused, "Not yours to see") {
		t.Fatalf("a permission refusal answered %d and did not name itself: %s", code, refused)
	}
	for _, wrong := range []string{"that session has ended", "sign in to open this page", "The bus is not answering"} {
		if strings.Contains(refused, wrong) {
			t.Errorf("a permission refusal reads as %q", wrong)
		}
	}
	// And a daemon that is not there is neither of the above.
	dead := httptest.NewServer(api.New(m.bus, m.tokens, "admin@h").Handler())
	deadWeb := httptest.NewServer(dashboard(&caller{client: dead.Client(), base: dead.URL}, false))
	defer deadWeb.Close()
	dead.Close()
	req, err := http.NewRequest("GET", deadWeb.URL+"/services", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(m.session)
	resp, err := deadWeb.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	offline, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway || !strings.Contains(string(offline), "The bus is not answering") {
		t.Fatalf("an unreachable bus answered %d and did not name itself: %s", resp.StatusCode, offline)
	}
	for _, wrong := range []string{"that session has ended", "sign in to open this page", "Not yours to see"} {
		if strings.Contains(string(offline), wrong) {
			t.Errorf("an unreachable bus reads as %q", wrong)
		}
	}
}

// Every page names itself in the tab and in history, and no two name
// themselves the same — including two of the same kind, which shared one
// generic title until this asked: two open Service tabs, two identities and
// two groups were indistinguishable. The shell travels with the page, so the
// same matrix carries the landmarks, the keyboard skip target, the account
// controls and the marked section, on every route rather than the handful a
// section check happens to walk.
func TestEverySignedInPageIsTitledUniquelyAndCarriesItsShell(t *testing.T) {
	m := meaningFixture(t)
	for _, who := range []string{"alice@h", "bob@h"} {
		if _, err := m.bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, group := range []string{"@ops", "@release"} {
		if err := m.bus.SetGroup("admin@h", group, []string{"alice@h"}); err != nil {
			t.Fatal(err)
		}
	}
	// Credentials with no user profile behind them, so the removal
	// confirmation is its own page rather than the refusal a directory user
	// gets for a credential that is not removable this way.
	for _, who := range []string{"#worker@h", "runner@h"} {
		if _, err := m.tokens.Issue(who); err != nil {
			t.Fatal(err)
		}
	}
	for _, record := range []protocol.Record{
		{Name: "service@h", Owner: "admin@h", Kind: protocol.KindService, Addr: "host:1", Proto: "https"},
		{Name: "second@h", Owner: "admin@h", Kind: protocol.KindService, Addr: "host:2", Proto: "https"},
		{Name: "#agent@h", Owner: "admin@h", Kind: "agent"},
		{Name: "channel@h", Owner: "admin@h", Kind: protocol.KindQueue},
		{Name: "other-channel@h", Owner: "admin@h", Kind: protocol.KindQueue},
		// A record using a group, so the group page draws its uses table.
		{Name: "ops-queue@h", Owner: "admin@h", Kind: protocol.KindQueue, Allow: []string{"@ops"}},
	} {
		m.register(record)
	}
	// The marked entry is the actual link for this page. The stylesheet also
	// contains the string, so looking for it anywhere passes with nothing
	// marked at all; "" is a page that is deliberately under no section.
	// Danger Zone, the confirmations and the record page take their section
	// from the record's own kind (`loadRecord`), so a Channel reaches them
	// too and can regress to Services on its own. `alias` is a second address
	// for a page already listed: it shares that page's title by definition,
	// so it is checked against it rather than counted as a collision.
	routes := []struct {
		path, current string
		alias         string
	}{
		{"/", "<a href=/ aria-current=page>", ""},
		{"/diagnostics", "<a href=/diagnostics aria-current=page>", ""},
		{"/services", "<a href=/services aria-current=page>", ""},
		{"/agents", "<a href=/agents aria-current=page>", ""},
		{"/personal", "<a href=/agents aria-current=page>", ""},
		{"/queues", "<a href=/queues aria-current=page>", ""},
		{"/pubsub", "<a href=/pubsub aria-current=page>", ""},
		{"/agents/new", "<a href=/agents aria-current=page>", ""},
		{"/services/new", "<a href=/services aria-current=page>", ""},
		{"/queues/new", "<a href=/queues aria-current=page>", ""},
		{"/pubsub/new", "<a href=/pubsub aria-current=page>", ""},
		{"/users", "<a href=/users aria-current=page>", ""},
		{"/users/new", "<a href=/users aria-current=page>", ""},
		{"/user?name=alice@h", "<a href=/users aria-current=page>", ""},
		{"/user?name=bob@h", "<a href=/users aria-current=page>", ""},
		{"/user-deactivate?name=alice@h", "<a href=/users aria-current=page>", ""},
		{"/user-deactivate?name=bob@h", "<a href=/users aria-current=page>", ""},
		{"/credential-remove?name=%23worker@h", "<a href=/users aria-current=page>", ""},
		{"/credential-remove?name=runner@h", "<a href=/users aria-current=page>", ""},
		{"/groups", "<a href=/groups aria-current=page>", ""},
		{"/groups/new", "<a href=/groups aria-current=page>", ""},
		{"/group?name=%40ops", "<a href=/groups aria-current=page>", ""},
		{"/group?name=%40release", "<a href=/groups aria-current=page>", ""},
		{"/activity", "<a href=/activity aria-current=page>", ""},
		{"/account", "<a class=account-link href=/account aria-current=page>", ""},
		{"/service?name=service@h", "<a href=/services aria-current=page>", ""},
		{"/service?name=second@h", "<a href=/services aria-current=page>", ""},
		// An agent belongs to the Agents section, whichever route reaches it.
		{"/agent?name=%23agent@h", "<a href=/agents aria-current=page>", ""},
		{"/service?name=%23agent@h", "<a href=/agents aria-current=page>", "Agent #agent@h · agent-bus"},
		{"/queue?name=channel@h", "<a href=/queues aria-current=page>", ""},
		{"/queue?name=other-channel@h", "<a href=/queues aria-current=page>", ""},
		// The old combined address is the same page.
		{"/queue?name=channel@h", "<a href=/queues aria-current=page>", "Queue channel@h · agent-bus"},
		{"/service-danger?name=service@h", "<a href=/services aria-current=page>", ""},
		{"/service-danger?name=second@h", "<a href=/services aria-current=page>", ""},
		{"/service-danger?name=%23agent@h", "<a href=/agents aria-current=page>", ""},
		{"/service-danger?name=channel@h", "<a href=/queues aria-current=page>", ""},
		// The retained legacy topic URL is the same page at its old address.
		{"/service?name=channel@h", "<a href=/queues aria-current=page>", "Queue channel@h · agent-bus"},
		{"/service?name=missing@h", "", ""},
	}
	seen := map[string]string{}
	tables := 0
	check := func(path, current, alias, body string) {
		t.Helper()
		title := section(t, body, "<title>", "</title>")
		if !strings.HasSuffix(title, " \u00b7 agent-bus") || strings.TrimSpace(strings.TrimSuffix(title, " \u00b7 agent-bus")) == "" {
			t.Errorf("%s is titled %q, which does not name a page of this node", path, title)
			return
		}
		if alias != "" {
			if title != alias {
				t.Errorf("%s is a second address for one page but is titled %q, not %q", path, title, alias)
			}
		} else if was, ok := seen[title]; ok {
			t.Errorf("%s and %s are both titled %q, so a tab and a history entry cannot tell them apart", was, path, title)
		} else {
			seen[title] = path
		}
		// Landmarks, once each, and the skip target they exist for.
		for want, n := range map[string]int{
			"<header class=site-header": 1, "<footer class=site-footer": 1,
			"<main>": 1, "</main>": 1,
			"<a class=skip-link href=#main>Skip to main content</a>": 1,
			"<a id=main tabindex=-1></a>":                            1,
		} {
			if got := strings.Count(body, want); got != n {
				t.Errorf("%s carries %s %d times, want %d", path, want, got, n)
			}
		}
		// Every table is named for assistive technology: a caption, or a
		// label naming its section (docs/05-discovery.md#browser-acceptance).
		for _, table := range unnamedTables(body) {
			t.Errorf("%s has a table without an accessible name: %s", path, table)
		}
		tables += strings.Count(body, "<table")
		// Sign-out and the way to one's own account belong beside the name
		// they act on, which means on the page, not on a chosen few. The
		// form's action alone is not the control: a missing button leaves it.
		for _, want := range []string{"<form method=post action=/signout", ">sign out</button>", "class=account-link href=/account"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s has no %s", path, want)
			}
		}
		if current == "" {
			if n := strings.Count(body, "aria-current=page>"); n != 0 {
				t.Errorf("%s marks %d section entries, and belongs under none", path, n)
			}
			return
		}
		if !strings.Contains(body, current) {
			t.Errorf("%s does not mark %s", path, current)
		}
		if n := strings.Count(body, "aria-current=page>"); n != 1 {
			t.Errorf("%s marks %d navigation entries, want 1", path, n)
		}
	}
	for _, route := range routes {
		body, _ := getAs(t, m, route.path)
		check(route.path, route.current, route.alias, body)
	}
	// Positive control for the table names: every table-drawing template
	// was reached, so the check above did not pass on pages without one.
	for _, want := range []string{"aria-labelledby=credentials", "aria-labelledby=loss",
		"aria-labelledby=leftovers", "aria-labelledby=group-uses", `aria-label="Slot values`} {
		found := false
		for _, route := range []string{"/account", "/diagnostics", "/group?name=%40ops", "/activity"} {
			if body, _ := getAs(t, m, route); strings.Contains(body, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("no page drew the table named by %s", want)
		}
	}
	if tables < 10 {
		t.Errorf("only %d tables were checked for a name", tables)
	}
	// A refusal names the refusal and not the name that was asked for, so two
	// not-found pages are deliberately one title: telling a hidden record from
	// an absent one is what the single answer exists to prevent.
	absent, _ := getAs(t, m, "/group?name=%40missing")
	if section(t, absent, "<title>", "</title>") != "No such name \u00b7 agent-bus" {
		t.Error("a second not-found page is titled apart from the first, which distinguishes hidden from absent")
	}

	// The confirmation pages are reachable only by posting the action they
	// confirm, and the brief counts them as pages.
	for _, action := range []struct {
		name, current string
		form          url.Values
	}{
		{"remove", "<a href=/services aria-current=page>", url.Values{"action": {"delete"}, "name": {"service@h"}}},
		{"transfer", "<a href=/services aria-current=page>", url.Values{"action": {"transfer"}, "name": {"second@h"}, "owner": {"alice@h"}}},
		// A queue reaches the same confirmation and belongs under Queues.
		{"channel remove", "<a href=/queues aria-current=page>", url.Values{"action": {"delete"}, "name": {"channel@h"}}},
		// So does an agent, and it belongs under Agents.
		{"agent remove", "<a href=/agents aria-current=page>", url.Values{"action": {"delete"}, "name": {"#agent@h"}}},
	} {
		body, code := postBody(t, m, "/service-confirm", action.form)
		if code != http.StatusOK {
			t.Fatalf("the %s confirmation answered %d: %s", action.name, code, body)
		}
		check("/service-confirm "+action.name, action.current, "", body)
	}
	// Positive control: the public page is the one without account controls,
	// so the loop above cannot be passing on strings the shell always carries.
	resp, err := m.client.Get(m.web.URL + "/signin")
	if err != nil {
		t.Fatal(err)
	}
	public, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(public), "action=/signout") || strings.Contains(string(public), "class=account-link") {
		t.Error("the public sign-in page carries account controls for a visitor who has no account")
	}
	if title := section(t, string(public), "<title>", "</title>"); title != "Sign in \u00b7 agent-bus" {
		t.Errorf("the public page is titled %q", title)
	}
}

// postBody is postAs for a page that answers with one: postAs returns the
// Location of a redirect, which a confirmation does not send.
func postBody(t *testing.T, m *meanings, path string, form url.Values) (string, int) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, m.web.URL+path, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(m.session)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", m.web.URL)
	resp, err := m.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return strings.Join(strings.Fields(string(body)), " "), resp.StatusCode
}

// A page whose own subject failed to load is a refusal, not a page that
// quietly says there is nothing registered. The recovery checks all fail the
// status call, which never reaches this second read.
func TestAFailedRecordReadIsARefusalNotAnEmptyRegistry(t *testing.T) {
	b := core.New()
	tokens, err := auth.Load(memory.NewTokens(), "admin@h")
	if err != nil {
		t.Fatal(err)
	}
	face := api.New(b, tokens, "admin@h").Handler()
	refuseList := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if refuseList && r.URL.Path == "/ls" {
			http.Error(w, `{"error":"the registry could not be read"}`, http.StatusInternalServerError)
			return
		}
		face.ServeHTTP(w, r)
	}))
	defer backend.Close()
	web := httptest.NewServer(dashboard(&caller{client: backend.Client(), base: backend.URL}, false))
	defer web.Close()
	client := web.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	token, err := tokens.Issue("admin@h")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.PostForm(web.URL+"/signin", url.Values{"token": {token}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	session := resp.Cookies()[0]
	get := func(path string) (int, string) {
		t.Helper()
		req, err := http.NewRequest("GET", web.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(session)
		r, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(body)
	}
	// Positive control: with the read answering, the page is the page.
	if code, body := get("/"); code != http.StatusOK || !strings.Contains(body, "Overview") {
		t.Fatalf("the healthy Overview answered %d: %s", code, body)
	}
	refuseList = true
	for _, path := range []string{"/", "/diagnostics"} {
		code, body := get(path)
		if code == http.StatusOK {
			t.Errorf("%s answered 200 with the registry unread, so nothing registered and nothing readable look alike", path)
		}
		// The sentence as the problem's detail, not inside its JSON envelope.
		if !strings.Contains(body, "<p class=warn>the registry could not be read</p>") {
			t.Errorf("%s does not carry the daemon's own reason: %s", path, body)
		}
	}
}

// unnamedTables returns each opening table tag that neither starts with a
// caption nor carries an aria-label or aria-labelledby.
func unnamedTables(body string) []string {
	var bad []string
	for rest := body; ; {
		i := strings.Index(rest, "<table")
		if i < 0 {
			return bad
		}
		rest = rest[i:]
		end := strings.Index(rest, ">")
		if end < 0 {
			return append(bad, rest)
		}
		tag := rest[:end+1]
		rest = rest[end+1:]
		if !strings.Contains(tag, "aria-label") && !strings.HasPrefix(rest, "<caption>") {
			bad = append(bad, tag)
		}
	}
}
