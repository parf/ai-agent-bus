package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// H.5.7: every service a paused or banned user owns refuses calls while that
// lasts (docs/01-identity.md#services-of-a-user-who-is-paused-or-banned). The
// check is on the called name, not on who is asking, so these are all about
// what `svc@h` answers rather than about who alice@h is.

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
	bus.Masters([]string{"admin@h"})
	for _, who := range []string{"alice@h", "maint@h", "bystander@h", "steady@h"} {
		if _, err := bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := bus.SetGroup("admin@h", core.MaintainersGroup, []string{"admin@h", "maint@h"}); err != nil {
		t.Fatal(err)
	}
	f := suspendFixture{bus, s, token, t}
	// alice owns the service under test; steady owns the positive control, so
	// every refusal below has a service beside it that answers throughout.
	f.call("alice@h", "POST", "/register", `{"name":"svc@h","allow":["alice@h","bystander@h","maint@h","admin@h"]}`, 200)
	f.call("steady@h", "POST", "/register", `{"name":"steady-svc@h","allow":["bystander@h"]}`, 200)
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
	f.call("admin@h", "POST", "/user/state", `{"name":"`+name+`","state":"`+state+`"}`, 200)
}

func (f suspendFixture) refusals() map[string]int {
	f.t.Helper()
	var st core.Status
	if err := json.Unmarshal([]byte(f.call("admin@h", "GET", "/status", "", 200)), &st); err != nil {
		f.t.Fatal(err)
	}
	return st.Refused
}

// The whole rule in one run: refused for everybody while it lasts, and serving
// again the moment it is lifted.
func TestASuspendedOwnersServiceRefusesEveryCaller(t *testing.T) {
	f := suspendedOwnerFixture(t)
	f.call("bystander@h", "POST", "/send", `{"to":"svc@h","body":"before"}`, 200)

	for _, state := range []string{"paused", "banned"} {
		f.state("alice@h", state)

		// A stranger on the ACL, a daemon maintainer, the daemon owner, and
		// the service's own principal. Not one of them is the suspended
		// person, which is the point: the check is on the called name.
		for _, who := range []string{"bystander@h", "maint@h", "admin@h"} {
			body := f.call(who, "POST", "/send", `{"to":"svc@h","body":"during"}`, 403)
			if !strings.Contains(body, "suspended") {
				t.Errorf("%s sending to a %s owner's service was refused without saying why: %s", who, state, body)
			}
		}
		// The service reading its own inbox, on its own credential.
		f.call("svc@h", "GET", "/consume?wait=0s", "", 403)

		// Not 404. The name exists and the caller may see it; answering
		// not-found would say it had never been registered.
		f.call("bystander@h", "GET", "/lookup?name=svc@h", "", 200)

		// Positive control, in the same state: an active owner's service is
		// untouched, so the refusal is about alice and not about the daemon.
		f.call("bystander@h", "POST", "/send", `{"to":"steady-svc@h","body":"unaffected"}`, 200)

		f.state("alice@h", "active")
		f.call("bystander@h", "POST", "/send", `{"to":"svc@h","body":"after"}`, 200)
		f.call("svc@h", "GET", "/consume?wait=0s", "", 200)
	}
}

// Nothing is destroyed by either state. This is the half that makes the rule
// liftable: a ban that reaped the work could not be undone.
func TestSuspensionDestroysNothing(t *testing.T) {
	f := suspendedOwnerFixture(t)
	f.call("bystander@h", "POST", "/send", `{"to":"svc@h","body":"queued before the pause"}`, 200)
	aliceHeld, svcHeld := f.token("alice@h"), f.token("svc@h")
	f.state("alice@h", "banned")

	// The record is still there, and still says what it said.
	var rec protocol.Record
	if err := json.Unmarshal([]byte(f.call("admin@h", "GET", "/lookup?name=svc@h", "", 200)), &rec); err != nil {
		t.Fatal(err)
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
			if u.State != "banned" {
				t.Errorf("the banned user's record does not say so: %+v", u)
			}
		}
	}
	if !found {
		t.Error("the ban removed the user record")
	}

	// Only the daemon owner lifts a ban: a maintainer who could pause cannot.
	f.call("maint@h", "POST", "/user/state", `{"name":"alice@h","state":"active"}`, 403)
	f.state("alice@h", "active")

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
// topic is how a third party reads a name that is not its own.
func TestAThirdPartyCannotReadASuspendedOwnersInbox(t *testing.T) {
	f := suspendedOwnerFixture(t)
	f.call("alice@h", "POST", "/register", `{"name":"jobs@h","kind":"topic","allow":["bystander@h","admin@h"],"share":true}`, 200)
	f.call("bystander@h", "POST", "/send", `{"to":"jobs@h","body":"waiting"}`, 200)
	f.state("alice@h", "paused")

	body := f.call("bystander@h", "GET", "/consume?topic=jobs@h&wait=0s", "", 403)
	if !strings.Contains(body, "suspended") {
		t.Errorf("a third-party read was refused without saying why: %s", body)
	}
	// The daemon owner's master access is read authority, and it does not
	// exempt them from this either.
	f.call("admin@h", "GET", "/consume?topic=jobs@h&wait=0s", "", 403)

	// The work is still there when the state is lifted; nothing was drained
	// or discarded while it was refused.
	f.state("alice@h", "active")
	if got := f.call("bystander@h", "GET", "/consume?topic=jobs@h&wait=0s", "", 200); !strings.Contains(got, "waiting") {
		t.Fatalf("the held message did not survive the pause: %s", got)
	}
}

// Joining a suspended owner's channel is a call to it; leaving one is not.
func TestASuspendedOwnersTopicRefusesNewSubscribersButLetsThemLeave(t *testing.T) {
	f := suspendedOwnerFixture(t)
	f.call("alice@h", "POST", "/register", `{"name":"feed@h","kind":"topic","mode":"pubsub","allow":["bystander@h","maint@h"]}`, 200)
	f.call("bystander@h", "POST", "/subscribe", `{"topic":"feed@h"}`, 200)
	f.state("alice@h", "paused")

	f.call("maint@h", "POST", "/subscribe", `{"topic":"feed@h"}`, 403)
	// Already subscribed, and free to go: trapping somebody in a channel they
	// can no longer use would be a worse answer than letting them leave.
	f.call("bystander@h", "POST", "/subscribe", `{"topic":"feed@h","off":true}`, 200)

	f.state("alice@h", "active")
	f.call("maint@h", "POST", "/subscribe", `{"topic":"feed@h"}`, 200)
}

// Suspension follows the record's stated owner and does not walk the chain.
// The contract is "every service they own", and ownership is the direct
// relation; a service may own a service, so the boundary is stated rather than
// assumed.
func TestSuspensionIsNotTransitive(t *testing.T) {
	f := suspendedOwnerFixture(t)
	f.call("alice@h", "POST", "/register", `{"name":"parent@h","allow":["bystander@h","parent@h"]}`, 200)
	f.call("parent@h", "POST", "/register", `{"name":"child@h","allow":["bystander@h"]}`, 200)
	f.state("alice@h", "paused")

	f.call("bystander@h", "POST", "/send", `{"to":"parent@h","body":"refused"}`, 403)
	// One hop further down and the person at the top is not this record's
	// owner any more. Reaching it would be a larger rule than the contract
	// states, and would need a cycle answer this does not have.
	f.call("bystander@h", "POST", "/send", `{"to":"child@h","body":"served"}`, 200)
}
