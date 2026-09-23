package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// listedRecord and listedUser are a listing's own row link for a name. A bare
// name passes on the page header, a flash or a search box; the link is the
// row. See CLAUDE.md, mutation first.
var nameInURL = strings.NewReplacer("#", "%23", "@", "%40")

func listedRecord(path, name string) string {
	return `class=record-name href="` + path + "?name=" + nameInURL.Replace(name) + "&"
}

func listedUser(name string) string {
	return `<a href="/user?name=` + nameInURL.Replace(name) + "&"
}

// A problem page carries the daemon's sentence, not the JSON envelope it came
// in, and a suspended caller is told it is suspended: "Not yours to see" says
// the thing is somebody else's, which is not why a suspended person is
// refused. See docs/05-discovery.md#refusals.
func TestASuspendedCallerIsToldSoInTheDaemonsOwnWords(t *testing.T) {
	m := meaningFixture(t)
	if _, err := m.bus.SetUser("admin@h", protocol.User{Name: "sleepy@h"}, true); err != nil {
		t.Fatal(err)
	}
	sleepy := m.as("sleepy@h")
	if _, err := m.bus.SetUserState("admin@h", "sleepy@h", protocol.StatusInactive); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("GET", m.web.URL+"/services", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(sleepy.session)
	resp, err := m.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	body := strings.Join(strings.Fields(string(raw)), " ")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a suspended caller answered %d, want 403: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, `<p class=warn>user access is suspended: sleepy@h is inactive</p>`) {
		t.Errorf("the problem page does not carry the daemon's sentence as its detail: %s", body)
	}
	if strings.Contains(body, "&#34;error&#34;") || strings.Contains(body, `"error"`) {
		t.Errorf("the problem page shows the raw error envelope: %s", body)
	}
	if !strings.Contains(body, `</svg> Your access is suspended</h1>`) {
		t.Errorf("the problem page does not name the suspension in its heading: %s", body)
	}
	if strings.Contains(body, "Not yours to see") {
		t.Errorf("a suspension reads as a permission refusal: %s", body)
	}
}

// The browser keeps the session id for as long as the bus does. A fixed
// Max-Age of the idle lifetime signed out a person still working thirty
// minutes after they signed in, which is the one thing an idle timeout exists
// not to do. See docs/05-discovery.md#signing-in.
func TestTheSessionCookieCarriesNoLifetimeOfItsOwn(t *testing.T) {
	m := meaningFixture(t)
	token, err := m.tokens.Issue("admin@h")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := m.client.PostForm(m.web.URL+"/signin", url.Values{"token": {token}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	set := resp.Header.Values("Set-Cookie")
	if len(set) != 1 || !strings.HasPrefix(set[0], cookieName+"=") {
		t.Fatalf("sign in set %q, want the one session cookie", set)
	}
	if lower := strings.ToLower(set[0]); strings.Contains(lower, "max-age") || strings.Contains(lower, "expires") {
		t.Errorf("the session cookie ends on a clock of its own rather than the bus's idle timeout: %s", set[0])
	}
}
