package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// On an account's own socket, the token of an Agent that account owns makes
// the request that Agent's — which is what lets a runner serve on the one
// socket forwarded to another host. Any other token, or none, leaves the
// request the account's (docs/02-access.md#local-socket).
func TestAnAccountSocketTakesItsOwnAgentsToken(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "owner@h")
	for _, name := range []string{"alice@h", "bob@h"} {
		if _, err := b.SetUser("owner@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	for name, owner := range map[string]string{"#mine@h": "alice@h", "#theirs@h": "bob@h"} {
		if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: name, Owner: owner}); err != nil {
			t.Fatal(err)
		}
	}
	alice, err := protocol.ParseName("alice@h")
	if err != nil {
		t.Fatal(err)
	}
	socket := s.HandlerFor(alice)
	you := func(tok string) string {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, "/status", strings.NewReader(""))
		if tok != "" {
			r.Header.Set(HeaderToken, tok)
		}
		w := httptest.NewRecorder()
		socket.ServeHTTP(w, r)
		var st struct {
			You string `json:"you"`
		}
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &st) != nil {
			t.Fatalf("status on the socket: %d %s", w.Code, w.Body.String())
		}
		return st.You
	}
	cases := []struct{ what, tok, want string }{
		{"no token", "", "alice@h"},
		{"her own agent's token", token("#mine@h"), "#mine@h"},
		{"another owner's agent's token", token("#theirs@h"), "alice@h"},
		{"another user's token", token("bob@h"), "alice@h"},
		{"a token nobody holds", "not-a-token", "alice@h"},
	}
	for _, c := range cases {
		if got := you(c.tok); got != c.want {
			t.Errorf("%s: the socket answered as %s, want %s", c.what, got, c.want)
		}
	}
}
