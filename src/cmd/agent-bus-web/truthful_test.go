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

// A page may not promise what the daemon will not do. Both of these were
// pages saying one thing while core said another: removal help claiming a
// credential survives when api.unregister forgets it, and a delete button for
// the one group core refuses to delete. See Plans/MVP/done/web-review.md W05
// and W08.
func TestPagesDoNotPromiseWhatTheDaemonRefuses(t *testing.T) {
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

	if _, err := b.Register(protocol.Record{Name: "admin@h", Owner: "admin@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "admin@h", Descr: "service"}); err != nil {
		t.Fatal(err)
	}
	// An ordinary group beside the protected one, so "no delete button" cannot
	// pass by there being no button anywhere.
	if err := b.SetGroup("admin@h", "@ops", []string{"admin@h"}); err != nil {
		t.Fatal(err)
	}

	get := func(path string) string {
		t.Helper()
		req, err := http.NewRequest("GET", web.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(session)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			t.Fatalf("%s: %d", path, resp.StatusCode)
		}
		return string(body)
	}

	service := get("/service-danger?name=svc@h")
	if strings.Contains(service, "Credentials remain valid") {
		t.Error("removal help still promises the credential survives; api.unregister forgets it")
	}
	if !strings.Contains(service, "No registration, no access") {
		t.Error("removal help does not say that removing the registration removes access")
	}
	if !strings.Contains(service, "goes with the address") {
		t.Error("removal help does not say the credential goes with the address")
	}

	// Every signed-in page carries the same shell. Checked on all of them,
	// because the duplicate nav is exactly how sign-out came to exist on one
	// page only (Plans/MVP/done/web-review.md W01).
	// The marked entry is checked as the actual link for this page: the
	// stylesheet also contains the string aria-current=page, so looking for it
	// anywhere on the page passes without any entry being marked at all.
	marks := map[string]string{
		"/": "<a href=/ aria-current=page>", "/services": "<a href=/services aria-current=page>",
		"/personal": "<a href=/services aria-current=page>", "/channels": "<a href=/channels aria-current=page>", "/users": "<a href=/users aria-current=page>",
		"/groups": "<a href=/groups aria-current=page>", "/activity": "<a href=/activity aria-current=page>", "/diagnostics": "<a href=/diagnostics aria-current=page>",
	}
	for _, path := range []string{"/", "/services", "/personal", "/channels", "/users", "/groups", "/activity", "/diagnostics"} {
		body := get(path)
		for _, want := range []string{
			`<html lang=en>`,
			`<meta name="viewport"`,
			`<main>`,
			`action=/signout`,
			marks[path],
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s has no %s", path, want)
			}
		}
		// And exactly one entry is marked, so "current" cannot mean "all".
		if n := strings.Count(body, "aria-current=page>"); n != 1 {
			t.Errorf("%s marks %d navigation entries, want 1", path, n)
		}
		if strings.Contains(body, "http-equiv=refresh") || strings.Contains(body, `http-equiv="refresh"`) {
			t.Errorf("%s still refreshes itself with no way to stop it", path)
		}
		if strings.Contains(body, "#888") {
			t.Errorf("%s still uses the muted grey that misses contrast", path)
		}
	}
	// Each page names itself, so a tab and a history entry can be told apart.
	titles := map[string]string{"/": "Overview", "/diagnostics": "Diagnostics", "/services": "Services", "/users": "Users", "/groups": "Groups"}
	for path, title := range titles {
		if want := "<title>" + title + " \u00b7 agent-bus</title>"; !strings.Contains(get(path), want) {
			t.Errorf("%s is not titled %q", path, title)
		}
	}

	groups := get("/groups")
	for _, group := range []string{"@administrators", "@ops"} {
		want := `<span role=img aria-label="Group">👥</span> <a href="/group?name=` + url.QueryEscape(group) + `"><code>` + group + `</code></a>`
		if !strings.Contains(groups, want) {
			t.Errorf("group %s has no group glyph immediately before its name", group)
		}
	}
	// Groups are not deleted at all, so no group offers it — the protected one
	// because core refuses it outright, the rest because deletion is not how a
	// group is retired. See docs/01-identity-and-roles.md#groups.
	if strings.Contains(groups, "value=delete") {
		t.Error("the groups page still offers a deletion")
	}
	// Positive control: the form itself is still there, so the check above
	// cannot pass by the page having lost its controls altogether.
	if detail := get("/group?name=%40ops"); !strings.Contains(detail, "value=save") {
		t.Error("the linked group detail lost membership editing, so the check above proves nothing")
	}
	for _, group := range []string{"@administrators", "@ops"} {
		if !strings.Contains(groups, group) {
			t.Errorf("%s is not on the page at all", group)
		}
	}
}

// A refusal is a page, not a dead end. Each of these was a bare http.Error
// body — no shell, no title, nobody's name on it and nowhere to go — or, on
// root, the sign-in form standing in for a bus that was not answering.
// See Plans/MVP/done/web-review.md W12.
func TestRefusalsRecoverInsteadOfDeadEnding(t *testing.T) {
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

	signIn := func(who string, extra url.Values) *http.Response {
		t.Helper()
		token, err := tokens.Issue(who)
		if err != nil {
			t.Fatal(err)
		}
		form := url.Values{"token": {token}}
		for k, v := range extra {
			form[k] = v
		}
		resp, err := client.PostForm(web.URL+"/signin", form)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	do := func(method, path string, session *http.Cookie, form url.Values) (int, string) {
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
			req.Header.Set("Origin", web.URL)
		}
		if session != nil {
			req.AddCookie(session)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	// An anonymous deep link enters sign-in at the address it asked for, and
	// carries that address in the form. It used to be `401 sign in required`
	// in plain text, with no form anywhere on it.
	code, body := do("GET", "/service?name=svc@h", nil, nil)
	if code != http.StatusUnauthorized {
		t.Errorf("an anonymous deep link answered %d, want 401", code)
	}
	if !strings.Contains(body, `action=/signin`) {
		t.Error("an anonymous deep link does not offer the sign-in form")
	}
	if !strings.Contains(body, `name=return value="/service?name=svc@h"`) {
		t.Errorf("the deep link is not carried back into the form: %s", body)
	}

	// And signing in from there goes to that page, not to the front page.
	resp := signIn("admin@h", url.Values{"return": {"/service?name=svc@h"}})
	resp.Body.Close()
	if to := resp.Header.Get("Location"); to != "/service?name=svc@h" {
		t.Errorf("sign-in returned to %q, not to where the deep link was going", to)
	}
	if len(resp.Cookies()) != 1 {
		t.Fatal("sign in did not return a session")
	}
	session := resp.Cookies()[0]

	// A return address that is not ours is refused, however it is dressed up.
	// The sign-in page is the one page a stranger can always reach, so a
	// return field that took a foreign URL would be an open redirect.
	// A backslash is in the list because several browsers read it as a slash,
	// so "/\\evil.example" is "//evil.example" to them: staying local means the
	// address the browser resolves, not the one the string looks like.
	for _, foreign := range []string{"https://evil.example/x", "//evil.example/x", "http:/\\evil.example", "javascript:alert(1)", "/\\evil.example/x", "https:/\\/\\evil.example"} {
		resp := signIn("admin@h", url.Values{"return": {foreign}})
		resp.Body.Close()
		to := resp.Header.Get("Location")
		where, err := url.Parse(to)
		if err != nil || where.Scheme != "" || where.Host != "" || !strings.HasPrefix(to, "/") || strings.HasPrefix(to, "//") || strings.ContainsAny(to, "\\") {
			t.Errorf("sign-in with return %q went to %q, which is not a page of ours", foreign, to)
		}
	}

	// A name that is not there, or is not one this caller may see, is one
	// answer, on the shell, with the navigation still under it.
	code, body = do("GET", "/service?name=nobody@h", session, nil)
	if code != http.StatusNotFound {
		t.Errorf("an unknown name answered %d, want 404", code)
	}
	for _, want := range []string{"No such name", "<main>", "action=/signout", "<title>Problem"} {
		if !strings.Contains(body, want) {
			t.Errorf("the not-found page has no %s: %s", want, body)
		}
	}

	// A permission refusal says so, and says that signing in again is not the
	// answer — which is exactly what the 401 page offers, so the two must not
	// read alike.
	known(t, b, "plain@h")
	plain := signIn("plain@h", nil)
	plain.Body.Close()
	if len(plain.Cookies()) != 1 {
		t.Fatal("the ordinary user did not get a session")
	}
	code, body = do("POST", "/groups", plain.Cookies()[0], url.Values{"action": {"save"}, "name": {"@ops"}, "members": {"plain@h"}})
	if code != http.StatusForbidden {
		t.Fatalf("an ordinary user editing a group answered %d, want 403: %s", code, body)
	}
	if !strings.Contains(body, "Not yours to see") {
		t.Errorf("a permission refusal is not presented as one: %s", body)
	}
	if strings.Contains(body, "action=/signin") {
		t.Error("a permission refusal offers signing in again, which cannot help")
	}

	// A bus that is not answering is its own page. This was the bad one: root
	// turned every failed status request into the anonymous page, so a stopped
	// daemon said "you are not signed in" and offered the one recovery that
	// could not work.
	dead := httptest.NewServer(api.New(b, tokens, "admin@h").Handler())
	deadWeb := httptest.NewServer(dashboard(&caller{client: dead.Client(), base: dead.URL}, false))
	defer deadWeb.Close()
	dead.Close()
	req, err := http.NewRequest("GET", deadWeb.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(session)
	down, err := deadWeb.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	offline, _ := io.ReadAll(down.Body)
	down.Body.Close()
	if down.StatusCode != http.StatusBadGateway {
		t.Errorf("an unreachable bus answered %d, want 502", down.StatusCode)
	}
	if !strings.Contains(string(offline), "The bus is not answering") {
		t.Errorf("an unreachable bus is not named as one: %s", offline)
	}
	if strings.Contains(string(offline), dead.URL) {
		t.Error("the recovery page exposes the backend address")
	}
	if strings.Contains(string(offline), "action=/signin") {
		t.Error("an unreachable bus is still presented as not being signed in")
	}
}
