package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func TestRequiredDashboardTabs(t *testing.T) {
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
	sign, err := client.PostForm(web.URL+"/signin", url.Values{"token": {token}})
	if err != nil {
		t.Fatal(err)
	}
	sign.Body.Close()
	session := sign.Cookies()[0]
	request := func(path string, form url.Values, want int) string {
		t.Helper()
		method := "GET"
		var reader io.Reader
		if form != nil {
			method = "POST"
			reader = strings.NewReader(form.Encode())
		}
		r, err := http.NewRequest(method, web.URL+path, reader)
		if err != nil {
			t.Fatal(err)
		}
		r.AddCookie(session)
		r.Header.Set("Origin", web.URL)
		if form != nil {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != want {
			t.Fatalf("%s: %d %s want %d", path, resp.StatusCode, body, want)
		}
		return string(body)
	}
	for _, path := range []string{"/services", "/personal", "/queues", "/pubsub", "/users", "/groups", "/activity"} {
		request(path, nil, 200)
	}
	request("/user", url.Values{"action": {"create"}, "name": {"alice@h"}, "person_name": {"Alice"}, "email": {"ALICE@example.com"}}, 303)
	refusedProfile := request("/user", url.Values{
		"action":      {"create"},
		"name":        {"duplicate@h"},
		"person_name": {"Duplicate & retained"},
		"email":       {"ALICE@example.com"},
		"github_user": {"duplicate-login"},
		"return":      {"/users?q=duplicate"},
		"token":       {"UNEXPECTED-PROFILE-SECRET"},
	}, 400)
	for _, retained := range []string{
		"Check this form",
		`<section class=form-error role=alert`,
		`href="#form-create"`,
		`value="duplicate@h" aria-invalid="false"`,
		`value="Duplicate &amp; retained"`,
		`value="ALICE@example.com"`,
		`value="duplicate-login"`,
		`value="/users?q=duplicate"`,
	} {
		if !strings.Contains(refusedProfile, retained) {
			t.Fatalf("refused profile lost safe input %q", retained)
		}
	}
	if strings.Contains(refusedProfile, `value="duplicate@h" aria-invalid="true"`) {
		t.Fatal("duplicate email was incorrectly attributed to the identity field")
	}
	if strings.Contains(refusedProfile, "UNEXPECTED-PROFILE-SECRET") || strings.Contains(refusedProfile, `&#34;error&#34;`) {
		t.Fatal("refused profile exposed an unrecognised field or raw API error")
	}
	if body := request("/users", nil, 200); !strings.Contains(body, "Alice") {
		t.Fatal("user was not listed")
	}
	if body := request("/user?name=alice@h", nil, 200); !strings.Contains(body, "alice@example.com") || !strings.Contains(body, ">Deactivate…</button>") ||
		!strings.Contains(body, "An Administrator may change an ordinary user; only the daemon Owner may change an Administrator") || strings.Contains(body, "lift a ban") {
		t.Fatal("missing profile or lifecycle controls")
	}
	request("/user", url.Values{"action": {"save"}, "name": {"alice@h"}, "person_name": {"Alice Updated"}, "email": {"alice@example.com"}}, 303)
	for _, state := range []string{protocol.StatusInactive, protocol.StatusActive} {
		request("/user", url.Values{"action": {state}, "name": {"alice@h"}}, 303)
		// The element form: every page's inline stylesheet names both classes.
		stateClass := `<span class="user-state user-state-` + state + `">`
		if body := request("/user?name=alice@h", nil, 200); !strings.Contains(body, stateClass) || !strings.Contains(body, "Alice Updated") {
			t.Fatalf("lifecycle change to %s failed or erased profile", state)
		}
	}
	// A retired state is refused rather than silently read as one of these.
	request("/user", url.Values{"action": {"banned"}, "name": {"alice@h"}}, 400)
	avatar := request("/avatar?name=alice@h", nil, 200)
	if !strings.Contains(avatar, "<svg") || !strings.Contains(avatar, ">A</text>") {
		t.Fatal("local avatar absent")
	}
	request("/service", url.Values{"action": {"create"}, "name": {"news@h"}, "kind": {"pubsub"}, "descr": {"News"}}, 303)
	refusedChannel := request("/service", url.Values{"action": {"create"}, "name": {"news@h"}, "kind": {"pubsub"}, "descr": {"Overwrite & retained"}, "allow": {"alice@h\n*"}}, 412)
	for _, retained := range []string{"Check this form", `<section class=form-error role=alert`, `href="#form-create"`, `value="news@h" aria-invalid="true"`, `value="Overwrite &amp; retained"`, ">alice@h\n*</textarea>"} {
		if !strings.Contains(refusedChannel, retained) {
			t.Fatalf("refused channel registration lost safe input %q", retained)
		}
	}
	// A refusal must not empty the other fields the same form asked for, and it
	// must come back on the page that was being registered. A service form that
	// lost the endpoint the daemon requires, or an agent refusal marked under
	// Services, is the face disagreeing with itself.
	refusedService := request("/service", url.Values{"action": {"create"}, "name": {"news@h"}, "kind": {"service"}, "addr": {"db.example:5432"}, "protocol": {"postgresql"}}, 412)
	for _, retained := range []string{`value="db.example:5432"`, `value="postgresql"`} {
		if !strings.Contains(refusedService, retained) {
			t.Fatalf("refused service registration lost its endpoint %q", retained)
		}
	}
	// news@h is refused as an agent's name: an agent's begins with #.
	refusedAgent := request("/service", url.Values{"action": {"create"}, "name": {"news@h"}, "kind": {"agent"}}, 400)
	if !strings.Contains(refusedAgent, `<a href=/agents aria-current=page>`) || strings.Count(refusedAgent, "aria-current=page>") != 1 {
		t.Fatal("a refused agent registration came back under another section")
	}
	// A User takes no published copy; the list delivers to one of its agents.
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#admin-box@h", Owner: "admin@h"}); err != nil {
		t.Fatal(err)
	}
	request("/service", url.Values{"action": {"save"}, "name": {"news@h"}, "edit_subs": {"1"}, "subs": {"#admin-box@h"}}, 303)
	rec, _ := b.Lookup("admin@h", "news@h")
	if len(rec.Subs) != 1 || rec.Subs[0] != "#admin-box@h" {
		t.Fatal("the Deliver-To editor did not write the list")
	}
	if body := request("/pubsub", nil, 200); !strings.Contains(body, "news@h") {
		t.Fatal("channel missing")
	}
	if body := request("/service?name=news@h", nil, 200); !strings.Contains(body, "value=remove-subscriber") {
		t.Fatal("the control that takes a recipient off the list is missing")
	}
	request("/service", url.Values{"action": {"remove-subscriber"}, "name": {"news@h"}, "subscriber": {"#admin-box@h"}}, 303)
	rec, _ = b.Lookup("admin@h", "news@h")
	if len(rec.Subs) != 0 {
		t.Fatal("removing a recipient failed")
	}
	b.SampleActivity(time.Now().Add(-time.Minute))
	if body := request("/activity", nil, 200); !strings.Contains(body, "<svg") || !strings.Contains(body, "Sample values") {
		t.Fatal("graphs and accessible values absent")
	}
}
