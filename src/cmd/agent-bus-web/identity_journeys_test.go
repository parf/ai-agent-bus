package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestAccountAndUserJourneysFollowDaemonFacts(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "alice@h", PersonName: "Alice"}, true); err != nil {
		t.Fatal(err)
	}
	if err := m.bus.SetGroup("admin@h", "@ops", []string{"alice@h"}); err != nil {
		t.Fatal(err)
	}
	if err := m.bus.SetGroup("admin@h", "@outer", []string{"@ops"}); err != nil {
		t.Fatal(err)
	}
	m.register(protocol.Record{Name: "alice-service@h", Owner: "alice@h", Kind: protocol.KindService, Addr: "host:1", Proto: "https"})
	m.register(protocol.Record{Name: "alice-channel@h", Owner: "alice@h", Kind: protocol.KindQueue, Allow: []string{"@ops"}})
	// An inbox beside a service under one owner, so the route each owned
	// record links to is a claim about its kind and not about the list.
	m.register(protocol.Record{Name: "alice-inbox@h", Owner: "alice@h", Kind: "agent"})
	m.register(protocol.Record{Name: "outer-service@h", Owner: "admin@h", Kind: protocol.KindAgent, Allow: []string{"@outer"}})
	m.register(protocol.Record{Name: "agent@h", Owner: "admin@h", Kind: "agent"})
	if _, err := m.tokens.Issue("agent@h"); err != nil {
		t.Fatal(err)
	}

	active := m.get("/user?name=alice@h")
	activeMain := section(t, active, "<main>", "</main>")
	for _, want := range []string{`href="/group?name=%40ops"`, ">Owned records</h2>", `href="/channel?name=alice-channel%40h"`, `href="/service?name=alice-service%40h"`, `href="/agent?name=alice-inbox%40h"`, `user-state-active`, `<details class=access-change>`, `>Change</summary>`, ">Pause</button>", ">Ban…</button>"} {
		if !strings.Contains(activeMain, want) {
			t.Errorf("active user detail lacks %q", want)
		}
	}
	for _, style := range []string{`.user-state-active{color:var(--green)}`, `.user-state-paused{color:var(--orange)}`, `.user-state-banned{color:var(--red)}`, `.access-change summary{color:var(--red)`} {
		if !strings.Contains(active, style) {
			t.Errorf("user access presentation lacks %q", style)
		}
	}
	if strings.Contains(activeMain, ">Activate</button>") || strings.Contains(activeMain, `name=action value=banned`) {
		t.Error("active user exposes an inapplicable transition or bypasses ban confirmation")
	}
	group := m.get("/group?name=%40ops")
	for _, want := range []string{`href="/channel?name=alice-channel%40h"`, "ACL via @outer", "outer-service@h"} {
		if !strings.Contains(group, want) {
			t.Errorf("group detail lacks caller-visible impact %q", want)
		}
	}
	ordinary := m.as("alice@h").get("/user?name=alice@h")
	if !strings.Contains(ordinary, ">Access</h2>") || !strings.Contains(ordinary, "user-state-active") || strings.Contains(ordinary, `class=access-change`) || strings.Contains(ordinary, ">Ban…</button>") {
		t.Fatal("ordinary user was shown administrative lifecycle controls")
	}
	confirm := m.get("/user-ban?name=alice@h")
	confirm = section(t, confirm, "<main>", "</main>")
	if !strings.Contains(confirm, "Confirm ban") || !strings.Contains(confirm, `name=action value=banned`) || strings.Contains(confirm, "user-state-banned") {
		t.Fatal("ban confirmation is absent, mutates state, or lacks its final action")
	}

	if _, err := m.bus.SetUserState("admin@h", "alice@h", "paused"); err != nil {
		t.Fatal(err)
	}
	paused := m.get("/user?name=alice@h")
	if !strings.Contains(paused, ">Activate</button>") || !strings.Contains(paused, ">Ban…</button>") || strings.Contains(paused, ">Pause</button>") {
		t.Fatal("paused user does not show exactly the applicable transitions")
	}
	if _, err := m.bus.SetUserState("admin@h", "alice@h", "banned"); err != nil {
		t.Fatal(err)
	}
	banned := m.get("/user?name=alice@h")
	if !strings.Contains(banned, ">Activate</button>") || strings.Contains(banned, ">Pause</button>") || strings.Contains(banned, ">Ban…</button>") {
		t.Fatal("banned user does not show exactly the applicable transition")
	}
	account := m.get("/account")
	if !strings.Contains(account, " Account</h1>") || !strings.Contains(account, `class=account-link href=/account aria-current=page`) || !strings.Contains(account, "Fingerprint") || !strings.Contains(account, "agent-bus-token admin@h --rotate") || !strings.Contains(account, "outer-service@h") {
		t.Fatal("profile-backed account lacks its identity or credential journey")
	}
	// Root was the diagnostics wall when this was written, so it asked "/".
	// Diagnostics is its own page now and is where a duplicate would come
	// back, which is what the failure below has always said.
	if strings.Contains(m.get("/diagnostics"), ">My names</h2>") {
		t.Fatal("credentials remain duplicated on Diagnostics")
	}
	if !strings.Contains(m.get("/services"), `class=account-link href=/account`) {
		t.Fatal("signed-in identity does not link to Account")
	}

	serviceAccount := m.as("agent@h").get("/account")
	for _, want := range []string{"This identity has a registered record", "Agent", "Fingerprint", ">Owner</th>", "<code>admin@h</code>"} {
		if !strings.Contains(serviceAccount, want) {
			t.Errorf("service-principal Account lacks %q", want)
		}
	}
}

func TestBanConfirmationStillRequiresTheDaemonAuthority(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "alice@h"}, true); err != nil {
		t.Fatal(err)
	}
	alice := m.as("alice@h")
	req, err := http.NewRequest(http.MethodGet, alice.web.URL+"/user-ban?name=alice@h", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(alice.session)
	resp, err := alice.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("ordinary user opened administrative confirmation: %d", resp.StatusCode)
	}

	// A forged final form is still refused by the daemon even when the browser
	// confirmation is skipped.
	form := url.Values{"action": {"banned"}, "name": {"alice@h"}}
	req, err = http.NewRequest(http.MethodPost, alice.web.URL+"/user", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(alice.session)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", alice.web.URL)
	resp, err = alice.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("forged lifecycle mutation was not refused: %d", resp.StatusCode)
	}
	users := m.bus.Users("admin@h", nil)
	for _, user := range users {
		if user.Name == "alice@h" && user.State != "active" {
			t.Fatal("refused forged ban changed the user")
		}
	}
}
