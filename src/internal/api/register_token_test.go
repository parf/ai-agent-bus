package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
)

// anyCode accepts whatever status a call is answered with.
const anyCode = -1

// A register that created an agent answers its credential, and nothing else
// does: the creating call is the one moment nobody else holds the name, so an
// Agent can start an agent for its User without the User's own credential,
// and a re-registration collects no existing agent's (runner M.10;
// docs/02-access.md#getting-a-token).

func TestARegisterThatCreatedAnAgentAnswersItsToken(t *testing.T) {
	b := core.New()
	s, tok := serverFor(t, b, "admin@h")
	do := func(token, path, body string, createOnly bool, want int) string {
		t.Helper()
		method := "POST"
		if body == "" {
			method = "GET"
		}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set(HeaderToken, token)
		if createOnly {
			r.Header.Set("If-None-Match", "*")
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if want != anyCode && w.Code != want {
			t.Fatalf("%s %s: %d %s, want %d", path, body, w.Code, w.Body.String(), want)
		}
		return w.Body.String()
	}
	answer := func(body string) (owner, token string, has bool) {
		t.Helper()
		var a map[string]any
		if err := json.Unmarshal([]byte(body), &a); err != nil {
			t.Fatal(err)
		}
		v, has := a["token"]
		token, _ = v.(string)
		owner, _ = a["owner"].(string)
		return owner, token, has
	}
	whoIs := func(token string) string {
		t.Helper()
		var st struct{ You string }
		if err := json.Unmarshal([]byte(do(token, "/status", "", false, 200)), &st); err != nil {
			t.Fatal(err)
		}
		return st.You
	}

	do(tok("admin@h"), "/user", `{"kind":"agent","name":"alice@h","person_name":"Alice","create":true}`, false, 200)
	alice := tok("alice@h")

	// The User creating an agent mints nothing: /token is theirs for it.
	if _, t1, has := answer(do(alice, "/register", `{"name":"#a1@h","kind":"agent"}`, false, 200)); has {
		t.Fatalf("a User creating #a1@h was answered a token %q", t1)
	}
	do(alice, "/register", `{"name":"#a1@h","kind":"agent"}`, true, 412)

	// Alice's agent — the runner — creating one for her, create-only.
	do(alice, "/register", `{"name":"#runner/alice@h","kind":"agent"}`, false, 200)
	var issued struct{ Token string }
	if err := json.Unmarshal([]byte(do(alice, "/token", `{"name":"#runner/alice@h"}`, false, 200)), &issued); err != nil {
		t.Fatal(err)
	}
	runner := issued.Token
	owner, child, has := answer(do(runner, "/register", `{"name":"#child@h","kind":"agent"}`, true, 200))
	if owner != "alice@h" {
		t.Fatalf("an agent created by alice's agent is owned by %q, not alice@h", owner)
	}
	if !has || child == "" || child == runner || child == alice {
		t.Fatalf("the agent creating #child@h was answered token %q (present %v)", child, has)
	}
	if you := whoIs(child); you != "#child@h" {
		t.Fatalf("the token answered for #child@h authenticates as %q", you)
	}
	// An agent re-registering itself on start, as every agent does, is
	// accepted and collects nothing: the record exists.
	if _, again, has := answer(do(child, "/register", `{"name":"#child@h","kind":"agent","descr":"started"}`, false, 200)); has {
		t.Fatalf("#child@h re-registering itself was answered a token %q", again)
	}
	// The agent re-registering an agent of its User's that exists — one it
	// created and one it did not — is answered no token, whether or not the
	// registration itself is allowed.
	for _, name := range []string{"#child@h", "#a1@h"} {
		body := do(runner, "/register", `{"name":"`+name+`","kind":"agent"}`, false, anyCode)
		if strings.Contains(body, `"token"`) {
			t.Fatalf("the agent re-registering %s collected a token: %s", name, body)
		}
	}
	// What is not an agent holds no credential, so a created queue answers none.
	if _, q, has := answer(do(runner, "/register", `{"name":"jobs@h","kind":"queue"}`, false, 200)); has {
		t.Fatalf("creating a queue answered a token %q", q)
	}
}
