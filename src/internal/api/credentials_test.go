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

	call("admin@h", "POST", "/user", `{"kind":"agent","name":"alice@h","person_name":"Alice","create":true}`, 200)
	call("alice@h", "POST", "/register", `{"name":"#svc@h","kind":"agent","descr":"a service"}`, 200)
	var minted struct{ Token string }
	if err := json.Unmarshal([]byte(call("alice@h", "POST", "/token", `{"kind":"agent","name":"#svc@h"}`, 200)), &minted); err != nil {
		t.Fatal(err)
	}
	svcToken := minted.Token
	if svcToken == "" {
		t.Fatal("no credential came back for #svc@h")
	}

	held := names("alice@h")
	svc, has := held["#svc@h"]
	if !has {
		t.Fatalf("a credential was issued for #svc@h but the holder is not told: %+v", held)
	}
	// Whose it is and what it is for: a name list that cannot tell a person
	// from a service cannot be read.
	if svc.Owner != "alice@h" || svc.Kind != "agent" {
		t.Fatalf("service credential does not say whose it is or what for: %+v", svc)
	}
	if me := held["alice@h"]; me.Kind != "person" || me.Owner != "alice@h" {
		t.Fatalf("a person's own credential is not marked as theirs: %+v", me)
	}

	call("alice@h", "POST", "/unregister", `{"kind":"agent","name":"#svc@h"}`, 200)

	// The credential goes with the address, and the test for that is whether
	// it still authenticates — not whether the name is listed. An
	// unregistered name is absent from the list either way, so asserting on
	// the list alone passes with the credential left intact.
	r := httptest.NewRequest("GET", "/status", nil)
	r.Header.Set(HeaderToken, svcToken)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code == 200 {
		t.Fatalf("the credential outlived the address it was for: /status said %s", w.Body.String())
	}

	held = names("alice@h")
	if leftover, still := held["#svc@h"]; still {
		t.Fatalf("a credential for a removed address is still listed: %+v", leftover)
	}
	// A person's own is not a service's: it is how they call at all, and
	// removing a record of theirs must not log them out.
	if me, mine := held["alice@h"]; !mine || me.Kind != "person" {
		t.Fatalf("unregistering a service took the person's own credential: %+v", held)
	}
}
