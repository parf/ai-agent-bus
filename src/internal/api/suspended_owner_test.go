package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// An inactive User makes every record it owns inactive, and an inactive record
// is no such entity to every caller: 404, not a separate suspended state
// (docs/constitution.md#-user). The User's own agents are refused as callers,
// 403 suspended. Nothing is destroyed, so reactivation restores everything.

type suspendFixture struct {
	bus   *core.Bus
	s     *Server
	token func(string) string
	t     *testing.T
}

func suspendedOwnerFixture(t *testing.T) suspendFixture {
	t.Helper()
	bus := core.New()
	s, token := serverFor(t, bus, "admin@h")
	for _, who := range []string{"alice@h", "maint@h", "bystander@h", "steady@h"} {
		if _, err := bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := bus.SetGroup("admin@h", core.AdministratorsGroup, []string{"admin@h", "maint@h"}); err != nil {
		t.Fatal(err)
	}
	f := suspendFixture{bus, s, token, t}
	// alice owns the service under test; steady owns the positive control, so
	// every refusal below has a service beside it that answers throughout.
	f.call("alice@h", "POST", "/register", `{"kind":"agent","name":"#svc@h","allow":["alice@h","bystander@h","maint@h","admin@h"]}`, 200)
	f.call("steady@h", "POST", "/register", `{"kind":"agent","name":"#steady-svc@h","allow":["bystander@h"]}`, 200)
	return f
}

func (f suspendFixture) call(who, method, path, body string, want int) string {
	f.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(HeaderToken, f.token(who))
	w := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(w, req)
	if w.Code != want {
		f.t.Fatalf("%s %s %s: got %d %s, want %d", who, method, path, w.Code, w.Body.String(), want)
	}
	return w.Body.String()
}

// raw uses exact credential bytes rather than asking the fixture for a token.
// f.call() goes through serverFor's helper, which calls Tokens.Issue every
// time, so a credential that had been forgotten would be silently recreated
// and "it still works" would be a fact about the fixture.
func (f suspendFixture) raw(token, method, path, body string, want int) string {
	f.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(HeaderToken, token)
	w := httptest.NewRecorder()
	f.s.Handler().ServeHTTP(w, req)
	if w.Code != want {
		f.t.Fatalf("%s %s on held credential: got %d %s, want %d", method, path, w.Code, w.Body.String(), want)
	}
	return w.Body.String()
}

func (f suspendFixture) state(name, state string) {
	f.t.Helper()
	f.call("admin@h", "POST", "/user/state", `{"kind":"agent","name":"`+name+`","status":"`+state+`"}`, 200)
}

func (f suspendFixture) refusals() map[string]int {
	f.t.Helper()
	var st core.Status
	if err := json.Unmarshal([]byte(f.call("admin@h", "GET", "/status", "", 200)), &st); err != nil {
		f.t.Fatal(err)
	}
	return st.Refused
}

// The whole rule in one run: no such record for everybody while it lasts, and
// serving again the moment the User is active.
func TestAnInactiveOwnersRecordsAreNoSuchEntity(t *testing.T) {
	f := suspendedOwnerFixture(t)
	f.call("bystander@h", "POST", "/send", `{"to":"#svc@h","body":"before"}`, 200)

	for _, state := range []string{"inactive"} {
		f.state("alice@h", state)

		// A stranger on the ACL, a daemon administrator and the daemon owner.
		// Not one of them is the inactive person, which is the point: the
		// called record is no such record, whoever asks.
		for _, who := range []string{"bystander@h", "maint@h", "admin@h"} {
			body := f.call(who, "POST", "/send", `{"to":"#svc@h","body":"during"}`, 404)
			if !strings.Contains(body, "no such receiver") {
				t.Errorf("%s sending to a %s owner's agent was not told there is no such receiver: %s", who, state, body)
			}
		}
		// The agent itself, on its own credential, is an inactive caller.
		// Push in the MCP face tells a suspension from every other 403 by this
		// sentence (src/mcp/push.ts), and waits it out instead of stopping.
		if body := f.call("#svc@h", "GET", "/consume?wait=0s", "", 403); !strings.Contains(body, "user access is suspended") {
			t.Errorf("an inactive caller's refusal does not say it is suspended: %s", body)
		}
		f.call("bystander@h", "GET", "/lookup?name=%23svc@h", "", 404)

		// Positive control, in the same state: an active owner's service is
		// untouched, so the refusal is about alice and not about the daemon.
		f.call("bystander@h", "POST", "/send", `{"to":"#steady-svc@h","body":"unaffected"}`, 200)

		f.state("alice@h", "active")
		f.call("bystander@h", "POST", "/send", `{"to":"#svc@h","body":"after"}`, 200)
		f.call("#svc@h", "GET", "/consume?wait=0s", "", 200)
	}
}

// Nothing is destroyed by inactivity. This is the half that makes the rule
// reversible: a deactivation that reaped the work could not be undone.
func TestSuspensionDestroysNothing(t *testing.T) {
	f := suspendedOwnerFixture(t)
	f.call("bystander@h", "POST", "/send", `{"to":"#svc@h","body":"queued before the pause"}`, 200)
	aliceHeld, svcHeld := f.token("alice@h"), f.token("#svc@h")
	f.state("alice@h", "inactive")

	// The record is still there, and still says what it said: hidden from
	// lookup, and in the daemon Owner's read-only view of inactive records.
	f.call("admin@h", "GET", "/lookup?name=%23svc@h", "", 404)
	var inactive []protocol.Record
	if err := json.Unmarshal([]byte(f.call("admin@h", "GET", "/inactive", "", 200)), &inactive); err != nil {
		t.Fatal(err)
	}
	var rec protocol.Record
	for _, r := range inactive {
		if r.Name == "#svc@h" {
			rec = r
		}
	}
	if rec.Owner != "alice@h" {
		t.Errorf("the ban changed the record's owner: %q", rec.Owner)
	}
	if rec.Queued != 1 {
		t.Errorf("the ban discarded the queue: %d held", rec.Queued)
	}
	// The owner's user record survives, in the state that was set.
	var people []protocol.User
	if err := json.Unmarshal([]byte(f.call("admin@h", "GET", "/users", "", 200)), &people); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, u := range people {
		if u.Name == "alice@h" {
			found = true
			if u.Status != "inactive" {
				t.Errorf("the banned user's record does not say so: %+v", u)
			}
		}
	}
	if !found {
		t.Error("the ban removed the user record")
	}

	// An Administrator may lift an ordinary user's ban. The credentials and
	// queue below were retained across that Administrator-authorized lift.
	f.call("maint@h", "POST", "/user/state", `{"kind":"agent","name":"alice@h","status":"active"}`, 200)

	// Both credentials still work, and these are the bytes held before the
	// ban rather than freshly minted ones, so this is kept-not-revoked rather
	// than reissued-on-demand.
	f.raw(aliceHeld, "GET", "/status", "", 200)
	if got := f.raw(svcHeld, "GET", "/consume?wait=0s", "", 200); !strings.Contains(got, "queued before the pause") {
		t.Fatalf("the queue did not survive the ban: %s", got)
	}
}

// The called-name check on the read path, which the service's own credential
// cannot certify: that one is refused at the gate and never reaches ConsumeAs.
// This reader is active, authorised, and somebody else entirely \u2014 a shared
// A channel is how a third party reads a name that is not its own.
func TestAThirdPartyCannotReadASuspendedOwnersInbox(t *testing.T) {
	f := suspendedOwnerFixture(t)
	f.call("alice@h", "POST", "/register", `{"name":"jobs@h","kind":"queue","allow":["bystander@h","admin@h"],"share":true}`, 200)
	f.call("bystander@h", "POST", "/send", `{"to":"jobs@h","body":"waiting"}`, 200)
	f.state("alice@h", "inactive")

	body := f.call("bystander@h", "GET", "/consume?inbox=jobs@h&wait=0s", "", 404)
	if !strings.Contains(body, "no inbox for jobs@h") {
		t.Errorf("a third-party read was not told there is no such inbox: %s", body)
	}
	// The daemon owner is explicitly listed, and the inactive owner still
	// makes the queue no such inbox to that otherwise-valid read authority.
	f.call("admin@h", "GET", "/consume?inbox=jobs@h&wait=0s", "", 404)

	// The work is still there when the state is lifted; nothing was drained
	// or discarded while it was refused.
	f.state("alice@h", "active")
	if got := f.call("bystander@h", "GET", "/consume?inbox=jobs@h&wait=0s", "", 200); !strings.Contains(got, "waiting") {
		t.Fatalf("the held message did not survive the pause: %s", got)
	}
}

// An inactive owner cannot write, and nobody names, its topic; it sends no
// copies meanwhile, so a recipient loses nothing by waiting, and leaves once
// the topic is back.
func TestAnInactiveOwnersTopicIsNamedByNobodyUntilItIsBack(t *testing.T) {
	f := suspendedOwnerFixture(t)
	// A published copy lands in an agent, never in a User's own inbox, so the
	// recipients are the two people's agents.
	f.call("bystander@h", "POST", "/register", `{"kind":"agent","name":"#bystander-box@h","allow":["*"]}`, 200)
	f.call("maint@h", "POST", "/register", `{"kind":"agent","name":"#maint-box@h","allow":["*"]}`, 200)
	f.call("alice@h", "POST", "/register", `{"name":"feed@h","kind":"pubsub","allow":["bystander@h","maint@h"],"subs":["#bystander-box@h"]}`, 200)
	f.state("alice@h", "inactive")

	f.call("alice@h", "POST", "/manage", `{"name":"feed@h","subs":["#bystander-box@h","#maint-box@h"]}`, 403)
	f.call("#bystander-box@h", "POST", "/subscribe", `{"channel":"feed@h","off":true}`, 404)
	f.call("maint@h", "POST", "/send", `{"to":"feed@h","body":"nobody hears"}`, 404)

	f.state("alice@h", "active")
	f.call("#bystander-box@h", "POST", "/subscribe", `{"channel":"feed@h","off":true}`, 200)
	f.call("alice@h", "POST", "/manage", `{"name":"feed@h","subs":["#maint-box@h"]}`, 200)
}

// Inactivity follows the record's User owner, and every record has one: what
// an agent registers is its User's, so a record made by an agent is inactive
// with that User too (docs/constitution.md#-registry-record).
func TestInactivityReachesWhatAnAgentMade(t *testing.T) {
	f := suspendedOwnerFixture(t)
	f.call("alice@h", "POST", "/register", `{"kind":"agent","name":"#parent@h","allow":["bystander@h","#parent@h"]}`, 200)
	f.call("#parent@h", "POST", "/register", `{"kind":"agent","name":"#child@h","allow":["bystander@h"]}`, 200)
	f.state("alice@h", "inactive")

	f.call("bystander@h", "POST", "/send", `{"to":"#parent@h","body":"refused"}`, 404)
	f.call("bystander@h", "POST", "/send", `{"to":"#child@h","body":"refused too"}`, 404)
}

// The reason is counted where the code is decided
// (docs/05-discovery.md#refusals), so a refusal nobody can see is not one.
func TestAnInactiveOwnersRefusalsAreCounted(t *testing.T) {
	f := suspendedOwnerFixture(t)
	f.state("alice@h", "inactive")

	// Two paths and two reasons, each counted. A caller naming the inactive
	// record is refused by a handler as unknown; the agent's own credential is
	// refused at the gate as suspended, and the gate hands the error to the
	// same reply() rather than answering for itself.
	//
	// Measured immediately around each request, and exactly one: a baseline
	// taken before the deactivation would also accept an increment at that
	// moment, or a double count, without proving this request produced one.
	for _, c := range []struct {
		who, method, path, body, reason string
		code                            int
	}{
		{"bystander@h", "POST", "/send", `{"to":"#svc@h","body":"counted"}`, "unknown", 404},
		{"#svc@h", "GET", "/status", "", "suspended", 403},
	} {
		before := f.refusals()[c.reason]
		f.call(c.who, c.method, c.path, c.body, c.code)
		if after := f.refusals()[c.reason]; after != before+1 {
			t.Errorf("%s %s: the %s counter went %d to %d, want one more", c.who, c.path, c.reason, before, after)
		}
	}
}
