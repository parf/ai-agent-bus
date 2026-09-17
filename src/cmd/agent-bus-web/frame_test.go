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
	for _, path := range []string{"/", "/services", "/personal", "/channels", "/service?name=svc@h", "/groups", "/users", "/user?name=admin@h", "/user?new=1", "/activity", "/service?name=missing@h"} {
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
			for _, fact := range []string{"AgentBus V" + version.String, "owner: <code>admin@h</code>", "<strong>uptime</strong>: "} {
				if !strings.Contains(header, fact) {
					t.Errorf("header lacks %q: %s", fact, header)
				}
			}
			if strings.Index(header, "owner:") > strings.Index(header, "<strong>uptime") || strings.Index(header, "<strong>uptime") > strings.Index(header, "<strong>calls") {
				t.Errorf("header identity and call fields are out of order: %s", header)
			}
			if !strings.Contains(header, `<div class=node-navigation>`) || !strings.Contains(header, `<nav aria-label="sections">`) {
				t.Error("navigation is not in the second header row")
			}
			footer := section(t, body, "<footer ", "</footer>")
			for _, fact := range []string{"Build: <code>" + version.Build} {
				if !strings.Contains(footer, fact) {
					t.Errorf("footer lacks %q: %s", fact, footer)
				}
			}
			if strings.Count(body, "<main>") != 1 || strings.Count(body, "</main>") != 1 {
				t.Error("page landmarks are unbalanced")
			}
		})
	}
}

func TestLoginUsesOnlyPublicDaemonFactsAndSeparatesBuilds(t *testing.T) {
	reads := 0
	broken := false
	legacy := false
	collecting := false
	matchingBuild := false
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
		if matchingBuild {
			json.NewEncoder(w).Encode(protocol.NodeIdentity{Version: version.String, Build: version.Build})
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
		for _, fact := range []string{"<svg class=node-logo", "AgentBus V9.8.7", "<strong>uptime</strong>: 1h23m", "owner&lt;&amp;&gt;@h", "<span>@ fixture-host</span>", "<strong>calls</strong>: minute: 7 ; hour: 0 ; total: 4321"} {
			if !strings.Contains(header, fact) {
				t.Errorf("header lacks %q: %s", fact, header)
			}
		}
		footer := section(t, body, "<footer ", "</footer>")
		for _, removed := range []string{"<details>", "About call counts", "Web <code>v", "minute:", "hour:", "total:"} {
			if strings.Contains(footer, removed) {
				t.Errorf("footer still contains %q: %s", removed, footer)
			}
		}
		if strings.Contains(header, "observed") || strings.Contains(header, "min:") || strings.Contains(header, "hr:") || strings.Contains(header, " up</span>") || strings.Contains(header, "build:") {
			t.Error("detail crept into header")
		}
		for _, old := range []string{"Host load", "About load readings", "accepted /", "dequeued"} {
			if strings.Contains(body, old) {
				t.Errorf("removed metric still on page: %s", old)
			}
		}
		if !strings.Contains(footer, "Daemon build: <code>daemon &lt;build&gt;") || !strings.Contains(footer, "Web build: <code>"+version.Build) || !strings.Contains(footer, version.Build) {
			t.Errorf("build sources confused or unescaped: %s", footer)
		}
		if !strings.Contains(body, "action=/signin") {
			t.Fatal("no sign-in form")
		}
	}
	if reads != 2 {
		t.Fatalf("%d public reads for two pages", reads)
	}
	matchingBuild = true
	matched := httptest.NewRecorder()
	h.ServeHTTP(matched, httptest.NewRequest("GET", "/", nil))
	matchedFooter := section(t, matched.Body.String(), "<footer ", "</footer>")
	if strings.Count(matchedFooter, "<code>") != 1 || !strings.Contains(matchedFooter, "Build: <code>"+version.Build+"</code>") || strings.Contains(matchedFooter, "Daemon build:") || strings.Contains(matchedFooter, "Web build:") {
		t.Fatalf("identical builds were not shown once: %s", matchedFooter)
	}
	matchingBuild = false
	collecting = true
	pending := httptest.NewRecorder()
	h.ServeHTTP(pending, httptest.NewRequest("GET", "/", nil))
	pendingHeader := section(t, pending.Body.String(), "<header ", "</header>")
	if !strings.Contains(pendingHeader, "minute: collecting history") || !strings.Contains(pendingHeader, "hour: collecting history") || !strings.Contains(pendingHeader, "total: 9") || strings.Contains(pendingHeader, "minute: 0") || strings.Contains(pendingHeader, "hour: 0") {
		t.Fatalf("unobserved windows fabricated a value: %s", pendingHeader)
	}
	collecting = false
	legacy = true
	old := httptest.NewRecorder()
	h.ServeHTTP(old, httptest.NewRequest("GET", "/", nil))
	oldHeader := section(t, old.Body.String(), "<header ", "</header>")
	for _, missing := range []string{"host unavailable", "<strong>calls</strong>: minute: unavailable; hour: unavailable; total: unavailable"} {
		if !strings.Contains(oldHeader, missing) {
			t.Errorf("missing legacy value fabricated: %s", oldHeader)
		}
	}
	legacy = false
	broken = true
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	header := section(t, w.Body.String(), "<header ", "</header>")
	if !strings.Contains(header, "Node information unavailable") || strings.Contains(header, "owner&lt;") || strings.Contains(header, version.String) {
		t.Fatalf("unavailable daemon was replaced by stale or local facts: %s", header)
	}
	footer := section(t, w.Body.String(), "<footer ", "</footer>")
	if !strings.Contains(footer, "Daemon build: unavailable") || !strings.Contains(footer, "Web build: <code>"+version.Build) {
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
	header := section(t, body, "<div class=node-summary>", "</div>")
	if !strings.Contains(header, "admin@h") || strings.Contains(header, "visitor@h") {
		t.Fatalf("owner is confused with visitor: %s", header)
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
