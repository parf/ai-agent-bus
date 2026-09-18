package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
)

func section(t *testing.T, page, start, end string) string {
	t.Helper()
	_, rest, ok := strings.Cut(page, start)
	if !ok {
		t.Fatalf("missing %s", start)
	}
	part, _, ok := strings.Cut(rest, end)
	if !ok {
		t.Fatalf("missing %s", end)
	}
	return part
}

func TestEveryHTMLPageCarriesNodeIdentity(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.Register(protocol.Record{Name: "svc@h", Owner: "admin@h"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/diagnostics", "/services", "/personal", "/channels", "/service?name=svc@h", "/groups", "/users", "/user?name=admin@h", "/user?new=1", "/activity", "/service?name=missing@h"} {
		t.Run(path, func(t *testing.T) {
			r := httptest.NewRequest("GET", path, nil)
			r.AddCookie(m.session)
			w := httptest.NewRecorder()
			dashboard(&caller{client: m.backend.Client(), base: m.backend.URL}, false).ServeHTTP(w, r)
			want := 200
			if strings.Contains(path, "missing") {
				want = 404
			}
			if w.Code != want {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			body := w.Body.String()
			header := section(t, body, "<header ", "</header>")
			if strings.Count(header, "<svg class=node-logo") != 1 || strings.Index(header, "</svg>") > strings.Index(header, "AgentBus") || !strings.Contains(header, `aria-hidden="true"`) {
				t.Fatalf("inline bus mark missing or not before its text label: %s", header)
			}
			for _, fact := range []string{"AgentBus <span class=build-tip tabindex=0 title=\"Build daemon " + version.Build + "\">v" + version.String + "</span>", "<span>@ parf.us</span>"} {
				if !strings.Contains(header, fact) {
					t.Errorf("header lacks %q: %s", fact, header)
				}
			}
			if !strings.Contains(header, `<div class=node-navigation>`) || !strings.Contains(header, `<nav aria-label="sections">`) {
				t.Error("navigation is not in the second header row")
			}
			footer := section(t, body, "<footer ", "</footer>")
			for _, fact := range []string{"<strong>Owner</strong> <code>admin@h</code>", "<strong>Uptime</strong>", "<strong>Calls</strong>"} {
				if !strings.Contains(footer, fact) {
					t.Errorf("footer lacks %q: %s", fact, footer)
				}
			}
			if strings.Index(footer, "<strong>Owner") > strings.Index(footer, "<strong>Uptime") || strings.Index(footer, "<strong>Uptime") > strings.Index(footer, "<strong>Calls") {
				t.Errorf("footer operational facts are out of order: %s", footer)
			}
			if strings.Count(body, "<main>") != 1 || strings.Count(body, "</main>") != 1 {
				t.Error("page landmarks are unbalanced")
			}
			if strings.Count(body, `<a class=skip-link href=#main>Skip to main content</a>`) != 1 ||
				strings.Count(body, `<a id=main tabindex=-1></a>`) != 1 {
				t.Error("page lacks one shared keyboard skip target")
			}
		})
	}
}

func TestLoginUsesOnlyPublicDaemonFactsAndSeparatesBuilds(t *testing.T) {
	reads := 0
	broken := false
	legacy := false
	collecting := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads++
		if r.URL.Path != "/identity" || r.Header.Get(api.HeaderToken) != "" || r.Header.Get("Cookie") != "" {
			t.Errorf("login used a private request: %s, headers %v", r.URL.Path, r.Header)
		}
		if broken {
			http.Error(w, "offline", 503)
			return
		}
		if collecting {
			json.NewEncoder(w).Encode(protocol.NodeIdentity{Calls: &protocol.CallStats{Total: 9, Windows: []protocol.CallWindow{{Window: "1m"}, {Window: "1h"}}}})
			return
		}
		if legacy {
			json.NewEncoder(w).Encode(protocol.NodeIdentity{Version: "9.8.7", Build: "old daemon build", Owner: "owner@h", Up: "1h23m"})
			return
		}
		json.NewEncoder(w).Encode(protocol.NodeIdentity{Version: "9.8.7", Build: "daemon <build>", Owner: "owner<&>@h", Up: "1h23m", Host: "fixture-host", Calls: &protocol.CallStats{Total: 4321, Windows: []protocol.CallWindow{{Window: "1m", Observed: "1m20s", Available: true, Count: 7}, {Window: "1h", Observed: "2m0s", Available: true}}}})
	}))
	defer backend.Close()
	h := dashboard(&caller{client: backend.Client(), base: backend.URL}, false)
	for _, path := range []string{"/", "/services"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		body := w.Body.String()
		header := section(t, body, "<header ", "</header>")
		for _, fact := range []string{"<svg class=node-logo", "AgentBus <span class=build-tip tabindex=0 title=\"Build daemon daemon &lt;build&gt;\">v9.8.7</span>", "<span>@ fixture-host</span>"} {
			if !strings.Contains(header, fact) {
				t.Errorf("header lacks %q: %s", fact, header)
			}
		}
		footer := section(t, body, "<footer ", "</footer>")
		for _, removed := range []string{"<details>", "About call counts", "Web <code>v", "Observed", "observed"} {
			if strings.Contains(footer, removed) {
				t.Errorf("footer still contains %q: %s", removed, footer)
			}
		}
		if strings.Contains(header, "observed") || strings.Contains(header, "minute:") || strings.Contains(header, "hour:") || strings.Contains(header, "uptime") || strings.Contains(header, "owner&lt;") {
			t.Error("detail crept into header")
		}
		for _, old := range []string{"Host load", "About load readings", "accepted /", "dequeued"} {
			if strings.Contains(body, old) {
				t.Errorf("removed metric still on page: %s", old)
			}
		}
		for _, fact := range []string{"<strong>Owner</strong> <code>owner&lt;&amp;&gt;@h</code>", "<strong>Uptime</strong> 1h23m", "<strong>Calls</strong> minute: 7; hour: 0; total: 4,321"} {
			if !strings.Contains(footer, fact) {
				t.Errorf("footer lacks %q: %s", fact, footer)
			}
		}
		if strings.Contains(footer, "development (unstamped)") || strings.Contains(footer, "; web ") || strings.Contains(footer, "<strong>Build</strong>") {
			t.Errorf("web implementation or a separate build item leaked into node facts: %s", footer)
		}
		if !strings.Contains(body, "action=/signin") {
			t.Fatal("no sign-in form")
		}
	}
	if reads != 2 {
		t.Fatalf("%d public reads for two pages", reads)
	}
	collecting = true
	pending := httptest.NewRecorder()
	h.ServeHTTP(pending, httptest.NewRequest("GET", "/", nil))
	pendingFooter := section(t, pending.Body.String(), "<footer ", "</footer>")
	if !strings.Contains(pendingFooter, "minute: collecting history") || !strings.Contains(pendingFooter, "hour: collecting history") || !strings.Contains(pendingFooter, "total: 9") || strings.Contains(pendingFooter, "minute: 0") || strings.Contains(pendingFooter, "hour: 0") {
		t.Fatalf("unobserved windows fabricated a value: %s", pendingFooter)
	}
	collecting = false
	legacy = true
	old := httptest.NewRecorder()
	h.ServeHTTP(old, httptest.NewRequest("GET", "/", nil))
	for _, missing := range []string{"AgentBus <span class=build-tip tabindex=0 title=\"Build daemon old daemon build\">v9.8.7</span>", "<strong>Calls</strong> minute: unavailable; hour: unavailable; total: unavailable"} {
		if !strings.Contains(old.Body.String(), missing) {
			t.Errorf("missing legacy value fabricated: %s", old.Body.String())
		}
	}
	legacy = false
	broken = true
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	header := section(t, w.Body.String(), "<header ", "</header>")
	if !strings.Contains(header, "node unavailable") || strings.Contains(header, "owner&lt;") || strings.Contains(header, version.String) {
		t.Fatalf("unavailable daemon was replaced by stale or local facts: %s", header)
	}
	footer := section(t, w.Body.String(), "<footer ", "</footer>")
	if strings.Contains(footer, "Build daemon") || strings.Contains(footer, version.Build) {
		t.Fatalf("missing daemon build borrowed a local value: %s", footer)
	}
	if !strings.Contains(w.Body.String(), "action=/signin") {
		t.Error("identity failure hid login")
	}
}

func TestNodeIdentityNamesTheDaemonOwnerNotTheVisitor(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "visitor@h"}, true); err != nil {
		t.Fatal(err)
	}
	body := m.as("visitor@h").get("/users")
	footer := section(t, body, "<footer ", "</footer>")
	if !strings.Contains(footer, "<strong>Owner</strong> <code>admin@h</code>") || strings.Contains(footer, "visitor@h") {
		t.Fatalf("owner is confused with visitor: %s", footer)
	}
	resp, err := m.client.Get(m.web.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || len(data) != 0 {
		t.Fatal("health check acquired page content")
	}
}
