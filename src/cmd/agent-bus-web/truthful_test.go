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
	if !strings.Contains(service, "No registration, no access") {
		t.Error("removal help does not say that removing the registration removes access")
	}
	if !strings.Contains(service, "goes with the address") {
		t.Error("removal help does not say the credential goes with the address")
	}

	groups := get("/groups")
	// Groups are not deleted at all, so no group offers it — the protected one
	// because core refuses it outright, the rest because deletion is not how a
	// group is retired. See docs/01-identity.md#groups-and-maintainers.
	if strings.Contains(groups, "value=delete") {
		t.Error("the groups page still offers a deletion")
	}
	// Positive control: the form itself is still there, so the check above
	// cannot pass by the page having lost its controls altogether.
	if !strings.Contains(groups, "value=save") {
		t.Error("the groups page lost membership editing, so the check above proves nothing")
	}
	for _, group := range []string{"@maintainers", "@ops"} {
		if !strings.Contains(groups, group) {
			t.Errorf("%s is not on the page at all", group)
		}
	}
}
