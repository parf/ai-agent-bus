package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestSelfProfileRouteCarriesOnlyEmail(t *testing.T) {
	b := core.New()
	enableGithubProfiles(b)
	s, tok := serverFor(t, b, "owner@h")
	for _, u := range []protocol.User{
		{Name: "alice@h", PersonName: "Alice", Email: "old@example.com", GithubUser: "alice-gh"},
		{Name: "bob@h", PersonName: "Bob", Email: "bob@example.com", GithubUser: "bob-gh"},
	} {
		if _, err := b.SetUser("owner@h", u, true); err != nil {
			t.Fatal(err)
		}
	}
	call := func(who, path, body string, want int) string {
		t.Helper()
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set(HeaderToken, tok(who))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d %s, want %d", who, path, w.Code, w.Body.String(), want)
		}
		return w.Body.String()
	}

	var got protocol.User
	if err := json.Unmarshal([]byte(call("alice@h", "/profile", `{"email":" New@Example.COM "}`, 200)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Email != "new@example.com" || got.PersonName != "Alice" || got.GithubUser != "alice-gh" || !got.CanSetEmail || got.CanEdit {
		t.Fatalf("bad self profile result: %+v", got)
	}
	call("alice@h", "/profile", `{"email":"BOB@example.com"}`, 400)
	for _, field := range []string{
		`"name":"alice@h"`,
		`"person_name":"Forged"`,
		`"github_user":"forged"`,
		`"state":"banned"`,
	} {
		call("alice@h", "/profile", `{"email":"other@example.com",`+field+`}`, 400)
	}
	// The full administrative operation does not become a second self-edit
	// door when the narrow capability is attached.
	call("alice@h", "/user", `{"kind":"agent","name":"alice@h","email":"other@example.com"}`, 403)

	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	call("#svc@h", "/profile", `{"email":"svc@example.com"}`, 404)
	if _, err := b.SetUserState("owner@h", "alice@h", "paused"); err != nil {
		t.Fatal(err)
	}
	call("alice@h", "/profile", `{"email":"paused@example.com"}`, 403)
}
