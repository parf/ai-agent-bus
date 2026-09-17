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
	for _, path := range []string{"/services", "/personal", "/channels", "/users", "/groups", "/activity"} {
		request(path, nil, 200)
	}
	request("/user", url.Values{"action": {"create"}, "name": {"alice@h"}, "person_name": {"Alice"}, "email": {"ALICE@example.com"}}, 303)
	if body := request("/users", nil, 200); !strings.Contains(body, "Alice") {
		t.Fatal("user was not listed")
	}
	if body := request("/user?name=alice@h", nil, 200); !strings.Contains(body, "alice@example.com") || !strings.Contains(body, ">Ban</button>") ||
		!strings.Contains(body, "Administrators may lift a ban on an ordinary user") || strings.Contains(body, "Only the daemon owner can lift a ban") {
		t.Fatal("missing profile or lifecycle controls")
	}
	request("/user", url.Values{"action": {"save"}, "name": {"alice@h"}, "person_name": {"Alice Updated"}, "email": {"alice@example.com"}}, 303)
	for _, state := range []string{"paused", "banned", "active"} {
		request("/user", url.Values{"action": {state}, "name": {"alice@h"}}, 303)
		if body := request("/user?name=alice@h", nil, 200); !strings.Contains(body, "State: "+state) || !strings.Contains(body, "Alice Updated") {
			t.Fatal("lifecycle change failed or erased profile")
		}
	}
	avatar := request("/avatar?name=alice@h", nil, 200)
	if !strings.Contains(avatar, "<svg") || !strings.Contains(avatar, ">A</text>") {
		t.Fatal("local avatar absent")
	}
	request("/service", url.Values{"action": {"create"}, "name": {"news@h"}, "kind": {"topic"}, "mode": {"pubsub"}, "descr": {"News"}}, 303)
	request("/service", url.Values{"action": {"create"}, "name": {"news@h"}, "kind": {"topic"}, "mode": {"pubsub"}, "descr": {"Overwrite"}}, 412)
	// Register the administrator's own inbox before subscribing it.
	request("/service", url.Values{"action": {"create"}, "name": {"admin@h"}, "kind": {"agent"}}, 303)
	request("/service", url.Values{"action": {"subscribe"}, "name": {"news@h"}}, 303)
	rec, _ := b.Lookup("admin@h", "news@h")
	if len(rec.Subs) != 1 || rec.Subs[0] != "admin@h" {
		t.Fatal("subscribe form failed")
	}
	if body := request("/channels", nil, 200); !strings.Contains(body, "news@h") {
		t.Fatal("channel missing")
	}
	if body := request("/service?name=news@h", nil, 200); !strings.Contains(body, "Remove subscription") {
		t.Fatal("subscription control missing")
	}
	request("/service", url.Values{"action": {"remove-subscriber"}, "name": {"news@h"}, "subscriber": {"admin@h"}}, 303)
	rec, _ = b.Lookup("admin@h", "news@h")
	if len(rec.Subs) != 0 {
		t.Fatal("remove subscription failed")
	}
	b.SampleActivity(time.Now().Add(-time.Minute))
	if body := request("/activity", nil, 200); !strings.Contains(body, "<svg") || !strings.Contains(body, "Sample values") {
		t.Fatal("graphs and accessible values absent")
	}
}
