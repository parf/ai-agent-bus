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
	if err := b.SetGroup("admin@h", "@ops", []string{"admin@h"}, false); err != nil {
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

	service := get("/service?name=svc@h")
	if strings.Contains(service, "Credentials remain valid") {
		t.Error("removal help still promises the credential survives; api.unregister forgets it")
	}
	if !strings.Contains(service, "goes with the address") {
		t.Error("removal help does not say the credential goes with the address")
	}

	groups := get("/groups")
	// The button is one form per group, so look inside the protected group's
	// own section rather than at the page.
	protected := groups[strings.Index(groups, "@maintainers"):]
	if end := strings.Index(protected, "</form>"); end >= 0 {
		protected = protected[:end]
	}
	if strings.Contains(protected, "value=delete") {
		t.Error("the maintainers group offers a delete core refuses unconditionally")
	}
	ops := groups[strings.Index(groups, "@ops"):]
	if end := strings.Index(ops, "</form>"); end >= 0 {
		ops = ops[:end]
	}
	if !strings.Contains(ops, "value=delete") {
		t.Error("an ordinary group lost its delete button, so the check above proves nothing")
	}
}
