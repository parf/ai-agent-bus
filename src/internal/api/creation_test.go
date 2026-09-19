package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

type publishedKeys struct{}

func (publishedKeys) Lookup(string) (ports.DirectoryEntry, error) {
	return ports.DirectoryEntry{Keys: []string{"ssh-ed25519 AAAAfixture"}}, nil
}

// Configuring a name that does not exist creates it, so it is a creation path
// and obeys what creation obeys. It did not: a name registration refused for
// being in a vouched realm could be taken by configuring it instead, and then
// issued a credential, with no key ever proved — in a realm whose whole point
// is that you prove one (docs/02-access.md#proving-possession).
func TestConfiguringDoesNotCreateWhatRegisteringRefuses(t *testing.T) {
	b := core.New()
	b.Directories(map[string]ports.Directory{"vouched": publishedKeys{}}, nil)
	s, token := serverFor(t, b, "admin@h")
	if _, err := b.SetUser("admin@h", protocol.User{Name: "ordinary@h"}, true); err != nil {
		t.Fatal(err)
	}
	call := func(who, path, body string) (int, string) {
		t.Helper()
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set(HeaderToken, token(who))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w.Code, strings.TrimSpace(w.Body.String())
	}

	// The gate registration applies, and the same gate by the other door.
	if code, body := call("ordinary@h", "/register", `{"kind":"agent","name":"victim@vouched"}`); code != 403 {
		t.Fatalf("registering into a vouched realm: %d %s, want 403", code, body)
	}
	if code, body := call("ordinary@h", "/configure", `{"kind":"agent","name":"victim@vouched","config":{"k":1}}`); code != 403 {
		t.Fatalf("configuring claimed a vouched name: %d %s, want 403", code, body)
	}
	if _, taken := b.Lookup("admin@h", "victim@vouched"); taken {
		t.Fatal("the refused configuration created the record anyway")
	}
	// And so the credential that would have followed is not there to mint.
	if code, body := call("admin@h", "/token", `{"kind":"agent","name":"victim@vouched"}`); code != 401 {
		t.Fatalf("a vouched name nobody proved was issued a credential: %d %s", code, body)
	}

	// Configuring still creates in an ordinary realm, which is the behaviour
	// this is not allowed to have broken.
	if code, body := call("ordinary@h", "/configure", `{"kind":"agent","name":"mine@h","config":{"k":1}}`); code != 200 {
		t.Fatalf("configuring could no longer create an ordinary name: %d %s", code, body)
	}
	if _, made := b.Lookup("ordinary@h", "mine@h"); !made {
		t.Fatal("configuring an ordinary name did not create it")
	}
	// And it still configures a record that already exists.
	if code, body := call("ordinary@h", "/configure", `{"kind":"agent","name":"mine@h","config":{"k":2}}`); code != 200 {
		t.Fatalf("reconfiguring an existing record: %d %s", code, body)
	}
}
