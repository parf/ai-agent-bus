package api

import (
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// H.5.10. A refusal that reaches a caller and no counter is a refusal the node
// cannot be asked about, and `status` claimed to carry every one of them
// (docs/05-discovery.md#refusals). Four paths answered through the bare writer.
//
// The fix is one counting place rather than four repairs, so these tests are
// about the observable rule: each refused request increments its reason once,
// while success, internal failure and router rejections do not.

type counted struct {
	t     *testing.T
	bus   *core.Bus
	srv   http.Handler
	as    func(string) string
	store *refusalStore
}

// A real token-store failure reaches the unmapped-error branch through HTTP.
// Enabled only after authentication is provisioned; no production test hook.
type refusalStore struct {
	ports.TokenStore
	err error
}

func (s *refusalStore) Save(creds []ports.Credential) error {
	if s.err != nil {
		return s.err
	}
	return s.TokenStore.Save(creds)
}

func refusalFixture(t *testing.T) *counted {
	t.Helper()
	bus := core.New()
	store := &refusalStore{TokenStore: memory.NewTokens()}
	tokens, err := auth.Load(store, "admin@h")
	if err != nil {
		t.Fatal(err)
	}
	s := New(bus, tokens, "admin@h")
	return &counted{t: t, bus: bus, srv: s.Handler(), store: store, as: func(name string) string {
		token, err := tokens.Issue(name)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}}
}

// call answers with the status, and what each reason moved by.
func (c *counted) call(method, path, body, token string) (int, map[string]int) {
	c.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set(HeaderToken, token)
	}
	w := httptest.NewRecorder()
	before := c.bus.Status().Refused
	c.srv.ServeHTTP(w, req)
	moved := c.bus.Status().Refused
	if moved == nil {
		moved = map[string]int{}
	}
	for reason, n := range before {
		moved[reason] -= n
	}
	for reason, n := range moved {
		if n == 0 {
			delete(moved, reason)
		}
	}
	return w.Code, moved
}

// one is the shape every counted refusal must have: the named reason up by
// exactly one, and nothing else moved. A test that checked only its own reason
// would pass a fix that counted every refusal under two.
func one(reason string) map[string]int { return map[string]int{reason: 1} }

func TestARefusalDecidedBeforeTheErrorMapStillCounts(t *testing.T) {
	c := refusalFixture(t)
	tok := c.as("admin@h")
	for _, call := range []struct {
		what, method, path, body, token string
		code                            int
		reason                          string
	}{
		{"an unparseable body", "POST", "/send", "{", tok, 400, "malformed"},
		{"an invalid name in a token request", "POST", "/token", `{"name":"!"}`, tok, 400, "malformed"},
		{"a lookup of a name the daemon does not hold", "GET", "/lookup?name=missing@h", "", tok, 404, "unknown"},
		// Unauthenticated, and counted for that reason rather than despite it:
		// an addressed endpoint's refusals count whatever the caller's
		// standing, which is the same rule that counts a bad token.
		{"an unparseable enrolment", "POST", "/enrol", "{", "", 400, "malformed"},
	} {
		code, moved := c.call(call.method, call.path, call.body, call.token)
		if code != call.code {
			t.Errorf("%s answered %d, want %d", call.what, code, call.code)
		}
		if !maps.Equal(moved, one(call.reason)) {
			t.Errorf("%s moved %v, want exactly %v", call.what, moved, one(call.reason))
		}
	}
}

// Already counted before this work, and still counted exactly once: a second
// counting layer would show up here as two.
func TestAlreadyCountedRefusalsAreNotCountedTwice(t *testing.T) {
	c := refusalFixture(t)
	tok := c.as("admin@h")
	for _, call := range []struct {
		what, path, body, token string
		reason                  string
		code                    int
	}{
		{"a bad token", "/ls", "", "not-a-token", "credential", 401},
		{"a send to a name nobody registered", "/send", `{"to":"missing@h","body":"x"}`, tok, "unknown", 404},
	} {
		method := "GET"
		if call.body != "" {
			method = "POST"
		}
		code, moved := c.call(method, call.path, call.body, call.token)
		if code != call.code {
			t.Errorf("%s answered %d, want %d", call.what, code, call.code)
		}
		if !maps.Equal(moved, one(call.reason)) {
			t.Errorf("%s moved %v, want exactly %v", call.what, moved, one(call.reason))
		}
	}
}

// A name hidden from this caller answers, and counts, exactly as a missing one
// does. Counting is a place a difference can leak out of, and the answer's
// whole point is that the two are indistinguishable.
func TestAHiddenNameCountsAsAMissingOneDoes(t *testing.T) {
	c := refusalFixture(t)
	if _, err := c.bus.SetUser("admin@h", protocol.User{Name: "stranger@h"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.bus.Register(protocol.Record{Name: "secret@h", Owner: "admin@h", Kind: protocol.KindTopic, Allow: []string{"admin@h"}, NoMaster: true}); err != nil {
		t.Fatal(err)
	}
	outsider := c.as("stranger@h")
	for _, name := range []string{"secret@h", "nothing@h"} {
		code, moved := c.call("GET", "/lookup?name="+name, "", outsider)
		if code != 404 || !maps.Equal(moved, one("unknown")) {
			t.Errorf("lookup %s answered %d, counted %v; want 404 and unknown once", name, code, moved)
		}
	}
}

// What must not count, which is the other half of the rule. A counter that
// rises on a success or on a route nothing served would make the figure
// useless in the other direction.
func TestWhatIsNotARefusalIsNotCounted(t *testing.T) {
	c := refusalFixture(t)
	tok := c.as("admin@h")
	// The consume below omits inbox and therefore reads the caller's own, so
	// the caller needs a record for that call to be an ordinary empty wait.
	if _, err := c.bus.Register(protocol.Record{Name: "admin@h", Owner: "admin@h"}); err != nil {
		t.Fatal(err)
	}
	for _, call := range []struct {
		what, method, path, body string
		code                     int
	}{
		{"a successful listing", "GET", "/ls", "", 200},
		{"a successful registration", "POST", "/register", `{"name":"fresh@h","owner":"admin@h"}`, 200},
		// Nothing arrives, so the wait ends empty. That is an answer, not a
		// refusal, and it is the commonest call this daemon serves.
		{"a consume that waits and finds nothing", "GET", "/consume?wait=1ms", "", 204},
		// No handler ran: the router turned it away, and the router refuses
		// nothing on anybody's behalf.
		{"a route the mux never matched", "GET", "/no-such-endpoint", "", 404},
		{"a method the mux refuses", "PUT", "/ls", "", 405},
		{"an unmatched topic filter", "GET", "/consume?topic=not-a-name&wait=1ms", "", 204},
		{"an address-shaped topic filter", "GET", "/consume?topic=missing@h&wait=1ms", "", 204},
	} {
		code, moved := c.call(call.method, call.path, call.body, tok)
		if code != call.code {
			t.Errorf("%s answered %d, want %d", call.what, code, call.code)
		}
		if len(moved) != 0 {
			t.Errorf("%s (answered %d) counted %v, want nothing", call.what, code, moved)
		}
	}
	// Rotate must persist a fresh credential. A store failure is an actual 500,
	// not an invented handler or an error injected into reply directly.
	c.store.err = errors.New("credential store unavailable")
	code, moved := c.call("POST", "/token", `{"name":"admin@h","rotate":true}`, tok)
	if code != 500 || len(moved) != 0 {
		t.Errorf("failed credential persistence answered %d, counted %v; want 500 and no refusal", code, moved)
	}
	c.store.err = nil
	code, moved = c.call("POST", "/token", `{"name":"admin@h","rotate":true}`, tok)
	if code != 200 || len(moved) != 0 {
		t.Errorf("recovered store answered %d, counted %v; want 200 and no refusal", code, moved)
	}
}

// A sampled refusal must use a reason in the table the dashboard reads.
func TestACountedRefusalUsesAReasonTheTableNames(t *testing.T) {
	c := refusalFixture(t)
	code, moved := c.call("GET", "/lookup?name=missing@h", "", c.as("admin@h"))
	if code != 404 || !maps.Equal(moved, one("unknown")) {
		t.Fatalf("fixture answered %d, counted %v; want 404 and unknown once", code, moved)
	}
	known := map[string]bool{}
	for _, reason := range Reasons() {
		known[reason] = true
	}
	for reason := range moved {
		if !known[reason] {
			t.Errorf("counted %q, which Reasons() does not name", reason)
		}
	}
}
