package main

import (
	"encoding/json"
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

func TestDashboardOwnerControls(t *testing.T) {
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
	sessions := map[string]*http.Cookie{}
	for _, who := range []string{"owner@h", "other@h", "admin@h"} {
		// Registered before signing in, not after: a name the daemon holds
		// nothing for but a credential cannot sign in at all
		// (docs/02-access.md#what-a-call-carries).
		known(t, b, who)
		token, err := tokens.Issue(who)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.PostForm(web.URL+"/signin", url.Values{"token": {token}})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 303 || len(resp.Cookies()) != 1 {
			t.Fatal("sign in did not return session")
		}
		sessions[who] = resp.Cookies()[0]
	}
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "owner@h", Descr: "service", Allow: []string{"owner@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Configure("svc@h", "owner@h", json.RawMessage(`{"secret":"NEVER-RENDER-THIS"}`)); err != nil {
		t.Fatal(err)
	}
	request := func(who, method, path, origin string, form url.Values, want int) string {
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
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if who != "" {
			req.AddCookie(sessions[who])
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != want {
			t.Fatalf("%s %s: %d %s, want %d", who, path, resp.StatusCode, body, want)
		}
		if resp.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("private page may be cached")
		}
		if strings.Contains(string(body), "NEVER-RENDER-THIS") {
			t.Fatal("private configuration rendered")
		}
		for _, session := range sessions {
			if strings.Contains(string(body), session.Value) {
				t.Fatal("credential rendered")
			}
		}
		return string(body)
	}
	request("", "GET", "/services", "", nil, 401)
	page := request("owner@h", "GET", "/service?name=svc@h", "", nil, 200)
	for _, label := range []string{"Save settings", "Replace configuration", "Transfer ownership", "Remove idle service"} {
		if !strings.Contains(page, ">"+label+"</button>") {
			t.Fatalf("missing owner control: %s", label)
		}
	}
	request("other@h", "GET", "/service?name=svc@h", "", nil, 404)
	if body := request("owner@h", "GET", "/services?scope=my&state=active", "", nil, 200); !strings.Contains(body, "svc@h") {
		t.Fatal("own active service missing")
	}
	disable := url.Values{"action": {"disable"}, "name": {"svc@h"}}
	request("owner@h", "POST", "/service", "https://evil.example", disable, 403)
	request("owner@h", "POST", "/service", "", disable, 403)
	request("other@h", "POST", "/service", web.URL, disable, 403)
	request("admin@h", "POST", "/service", web.URL, disable, 403)
	request("owner@h", "POST", "/service", web.URL, disable, 303)
	if body := request("owner@h", "GET", "/services?scope=my&state=active", "", nil, 200); strings.Contains(body, "svc@h") {
		t.Fatal("disabled service appears active")
	}
	if body := request("owner@h", "GET", "/services?scope=my&state=inactive", "", nil, 200); !strings.Contains(body, "svc@h") {
		t.Fatal("disabled service missing")
	}
	group := url.Values{"action": {"save"}, "name": {"@ops"}, "members": {"other@h"}}
	request("owner@h", "POST", "/groups", web.URL, group, 403)
	request("admin@h", "POST", "/groups", web.URL, group, 303)
	request("owner@h", "POST", "/service", web.URL, url.Values{"action": {"maintainers"}, "name": {"svc@h"}, "maintainers": {"@ops"}}, 303)
	page = request("other@h", "GET", "/service?name=svc@h", "", nil, 200)
	if !strings.Contains(page, "Save settings") || strings.Contains(page, "Transfer ownership") {
		t.Fatal("maintainer controls wrong")
	}
	request("other@h", "POST", "/service", web.URL, url.Values{"action": {"enable"}, "name": {"svc@h"}}, 303)
	request("other@h", "POST", "/service", web.URL, url.Values{"action": {"transfer"}, "name": {"svc@h"}, "owner": {"other@h"}}, 403)
	request("owner@h", "POST", "/service", web.URL, url.Values{"action": {"save"}, "name": {"svc@h"}, "descr": {"updated"}, "allow": {"owner@h"}, "bound": {"3"}, "overflow": {"strict"}}, 303)
	record, _ := b.Lookup("owner@h", "svc@h")
	if record.Descr != "updated" || record.Bound != 3 {
		t.Fatal("form did not change daemon record")
	}
	request("owner@h", "POST", "/service", web.URL, url.Values{"action": {"configure"}, "name": {"svc@h"}, "config": {`{"secret":"NEVER-RENDER-THIS","new":true}`}}, 303)
	request("owner@h", "POST", "/service", web.URL, url.Values{"action": {"transfer"}, "name": {"svc@h"}, "owner": {"other@h"}}, 303)
	request("owner@h", "POST", "/service", web.URL, disable, 403)
	request("other@h", "POST", "/service", web.URL, url.Values{"action": {"delete"}, "name": {"svc@h"}}, 303)
	if _, ok := b.Lookup("other@h", "svc@h"); ok {
		t.Fatal("delete form did not unregister")
	}
}

func TestSameOrigin(t *testing.T) {
	for _, test := range []struct {
		origin, site string
		want         bool
	}{
		{"https://bus.local:8443", "same-origin", true},
		{"http://bus.local:8443", "same-origin", false},
		{"https://bus.local", "same-origin", false},
		{"https://bus.local:8443", "cross-site", false},
		{"https://bus.local:8443/evil", "", false},
		{"https://attacker@bus.local:8443", "", false},
		{"null", "", false},
	} {
		r := httptest.NewRequest("POST", "https://bus.local:8443/service", nil)
		r.Header.Set("Origin", test.origin)
		r.Header.Set("Sec-Fetch-Site", test.site)
		if sameOrigin(r, true) != test.want {
			t.Errorf("origin %s site %s", test.origin, test.site)
		}
	}
}
