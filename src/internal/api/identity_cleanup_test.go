package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestIdentityCleanupRechecksAuthorityAndCurrentState(t *testing.T) {
	b := core.New()
	s, token := serverFor(t, b, "owner@h")
	if err := b.SetGroup("owner@h", core.AdministratorsGroup, []string{"owner@h", "maintainer@h"}); err != nil {
		t.Fatal(err)
	}
	call := func(who, method, path, body string, want int) []byte {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set(HeaderToken, token(who))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s as %s: HTTP %d, want %d", method, path, who, w.Code, want)
		}
		return w.Body.Bytes()
	}
	previous := token("unused@h")
	current, err := s.tokens.Rotate("unused@h")
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.tokens.StartSession("unused@h")
	if err != nil {
		t.Fatal(err)
	}
	unrelated := token("unrelated@h")
	for _, who := range []string{"owner@h", "maintainer@h"} {
		var rows []protocol.User
		if err := json.Unmarshal(call(who, "GET", "/users", "", 200), &rows); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range rows {
			if row.Name == "unused@h" {
				found = row.Kind == protocol.DirectoryCredential && row.CanRemove
			}
		}
		if !found {
			t.Fatalf("%s cannot review the eligible credential", who)
		}
	}
	// GETs did not clean anything. An ordinary principal cannot invoke the
	// mutation directly, even for its own otherwise eligible credential.
	//
	// Two refusals, because there are two kinds of ordinary caller. The
	// eligible credential is by definition a name the daemon holds nothing
	// else for, so it no longer reaches a handler at all
	// (docs/02-access.md#what-a-call-carries) — and a registered principal,
	// which does reach one, is refused there for not being a maintainer. The
	// first alone would stop pinning the authorization check.
	call("unused@h", "POST", "/identity/remove", `{"kind":"agent","name":"unused@h"}`, 401)
	known(t, b, "ordinary@h")
	call("ordinary@h", "POST", "/identity/remove", `{"kind":"agent","name":"unused@h"}`, 403)
	if _, ok := s.tokens.Principal(current); !ok {
		t.Fatal("reading or denied removal changed the credential")
	}
	call("maintainer@h", "POST", "/identity/remove", `{"kind":"agent","name":"unused@h"}`, 200)
	for label, value := range map[string]string{"current": current, "previous": previous, "session": session} {
		if _, ok := s.tokens.Principal(value); ok {
			t.Errorf("%s still authenticates after manual cleanup", label)
		}
	}
	if _, ok := s.tokens.Principal(unrelated); !ok {
		t.Error("cleanup affected unrelated credentials")
	}
	// The user looked at an eligible row, but something registered before
	// submission. Each independently protective condition must be rechecked.
	// "owns-services" was a third shape here until orphan deletion landed. It
	// is gone rather than relaxed: the state it built — a credential-only name
	// still owning records, from an older store — is deleted at startup now
	// (docs/01-identity-and-roles.md#orphaned-records), so no serving daemon has
	// one for the recheck to meet.
	for _, shape := range []string{"user", "record"} {
		name := shape + "@h"
		cred := token(name)
		call("owner@h", "GET", "/users", "", 200)
		switch shape {
		case "user":
			if _, err := b.SetUser("owner@h", protocol.User{Name: name}, true); err != nil {
				t.Fatal(err)
			}
			if err := b.Unregister(name, name); err != nil {
				t.Fatal(err)
			} // profile alone protects it
		case "record":
			if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: name, Owner: "owner@h"}); err != nil {
				t.Fatal(err)
			}
		}
		call("owner@h", "POST", "/identity/remove", `{"kind":"agent","name":"`+name+`"}`, 409)
		if _, ok := s.tokens.Principal(cred); !ok {
			t.Errorf("stale directory row removed %s", shape)
		}
	}
	call("owner@h", "POST", "/identity/remove", `{"kind":"agent","name":"owner@h"}`, 409)
	call("owner@h", "POST", "/identity/remove", `{"kind":"agent","name":"maintainer@h"}`, 409)
	if counts := b.Status().Refused; counts["busy"] != 4 || counts["malformed"] != 0 {
		t.Fatalf("valid cleanup conflicts counted as malformed requests: %v", counts)
	}
	call("owner@h", "POST", "/identity/remove", `{"kind":"agent","name":"unrelated@h"}`, 200)
	if _, ok := s.tokens.Principal(unrelated); ok {
		t.Error("owner cleanup did not remove eligible credential")
	}
}
