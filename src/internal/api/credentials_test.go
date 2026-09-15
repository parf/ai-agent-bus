package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
)

// A name list that cannot say whose a credential is, or what it is for,
// cannot be read: one run of the launcher smoke left ~250 rows that all
// looked alike. The credential itself outlives the address on purpose — it is
// what lets the holder come back and what stops a stranger taking the name —
// so a leftover is labelled rather than removed (docs/05-discovery.md#dashboard).
func TestNamesSayWhoseCredentialAndWhatFor(t *testing.T) {
	b := core.New()
	s, tok := serverFor(t, b, "admin@h")
	call := func(who, method, path, body string, want int) string {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set(HeaderToken, tok(who))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d %s, want %d", who, path, w.Code, w.Body.String(), want)
		}
		return w.Body.String()
	}
	names := func(who string) map[string]auth.Held {
		t.Helper()
		var held []auth.Held
		if err := json.Unmarshal([]byte(call(who, "GET", "/names", "", 200)), &held); err != nil {
			t.Fatal(err)
		}
		out := map[string]auth.Held{}
		for _, h := range held {
			out[h.Name] = h
		}
		return out
	}

	call("admin@h", "POST", "/user", `{"name":"alice@h","person_name":"Alice","create":true}`, 200)
	call("alice@h", "POST", "/register", `{"name":"svc@h","kind":"agent","descr":"a service"}`, 200)
	call("alice@h", "POST", "/token", `{"name":"svc@h"}`, 200)

	held := names("alice@h")
	svc, has := held["svc@h"]
	if !has {
		t.Fatalf("a credential was issued for svc@h but the holder is not told: %+v", held)
	}
	// Whose it is and what it is for: a name list that cannot tell a person
	// from a service cannot be read.
	if svc.Owner != "alice@h" || svc.Kind != "agent" {
		t.Fatalf("service credential does not say whose it is or what for: %+v", svc)
	}
	if me := held["alice@h"]; me.Kind != "person" || me.Owner != "alice@h" {
		t.Fatalf("a person's own credential is not marked as theirs: %+v", me)
	}

	call("alice@h", "POST", "/unregister", `{"name":"svc@h"}`, 200)

	held = names("alice@h")
	// Still held, by design. What changes is that it is now visibly a name
	// nothing answers on, instead of sitting in the list looking live.
	gone, still := held["svc@h"]
	if !still {
		t.Fatalf("removing an address took the credential that reclaims it: %+v", held)
	}
	if gone.Kind != "unregistered" {
		t.Fatalf("a credential for a removed address is not marked as a leftover: %+v", gone)
	}
	if me := held["alice@h"]; me.Kind != "person" {
		t.Fatalf("a person's own credential changed when their service went: %+v", me)
	}
}
