package api

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A group is retired by emptying its membership
// (docs/01-identity-and-roles.md#groups). These are H.5.6's checks: the
// deletion verb is gone from the API as well as the page, and emptying does
// what deletion was reached for without the damage deletion would have done.
func groupFixture(t *testing.T) (*core.Bus, *Server, func(string) string) {
	t.Helper()
	bus := core.New()
	s, token := serverFor(t, bus, "admin@h")
	for _, who := range []string{"alice@h", "maint@h", "plain@h"} {
		if _, err := bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	return bus, s, token
}

func post(t *testing.T, s *Server, token func(string) string, who, path, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set(HeaderToken, token(who))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// Refused whoever sends it: the daemon owner has no more of this verb than an
// ordinary user, because it is not an authority question.
func TestNoPathRemovesAGroup(t *testing.T) {
	bus, s, token := groupFixture(t)
	if err := bus.SetGroup("admin@h", "@ops", []string{"maint@h"}); err != nil {
		t.Fatal(err)
	}
	for _, who := range []string{"admin@h", "maint@h", "plain@h"} {
		code, body := post(t, s, token, who, "/group", `{"kind":"agent","name":"@ops","remove":true}`)
		if code != 400 {
			t.Errorf("%s asking to remove a group answered %d, want 400: %s", who, code, body)
		}
	}
	// Carrying members alongside the removal does not get them applied on the
	// way out: a refused request does nothing at all, rather than doing the
	// save it was not asked for.
	if code, body := post(t, s, token, "admin@h", "/group", `{"kind":"agent","name":"@ops","remove":true,"members":["plain@h"]}`); code != 400 {
		t.Errorf("a removal carrying members answered %d, want 400: %s", code, body)
	}
	// The group is still there, and still has the member it had. A removal
	// request that decoded as "set the members to none" would leave the same
	// 200-shaped world as a deliberate emptying, which is the silent
	// reinterpretation this refusal exists to prevent.
	//
	// The fixture matters: @ops is nonempty and named by no record, which is
	// exactly the case the old deletion branch allowed. A referenced group or
	// @administrators would have been refused with the verb still present, so
	// either would pass this test against the code it is meant to reject.
	members := bus.Groups("admin@h")["@ops"]
	if len(members) != 1 || members[0] != "maint@h" {
		t.Fatalf("a refused removal changed the membership: %v", members)
	}
}

// An emptied group is a group, and survives a restart as one. Retirement that
// did not persist would be deletion with a delay: the name would come back
// unmapped and the records naming it would be pointing at nothing.
func TestAnEmptiedGroupSurvivesASnapshotRoundTrip(t *testing.T) {
	bus, s, token := groupFixture(t)
	if err := bus.SetGroup("admin@h", "@ops", []string{"maint@h"}); err != nil {
		t.Fatal(err)
	}
	if code, body := post(t, s, token, "alice@h", "/register", `{"kind":"agent","name":"svc@h","allow":["alice@h","@ops"]}`); code != 200 {
		t.Fatalf("register answered %d: %s", code, body)
	}
	if code, body := post(t, s, token, "alice@h", "/manage", `{"kind":"agent","name":"svc@h","maintainers":"@ops"}`); code != 200 {
		t.Fatalf("assigning maintainers answered %d: %s", code, body)
	}
	if code, body := post(t, s, token, "admin@h", "/group", `{"kind":"agent","name":"@ops","members":[]}`); code != 200 {
		t.Fatalf("emptying answered %d: %s", code, body)
	}

	restarted := core.New()
	restarted.Restore(bus.Snapshot())
	members, ok := restarted.Groups("admin@h")["@ops"]
	if !ok {
		t.Fatal("the emptied group did not survive the snapshot")
	}
	if len(members) != 0 {
		t.Errorf("the emptied group came back with members: %v", members)
	}
	rec, found := restarted.Lookup("alice@h", "svc@h")
	if !found {
		t.Fatal("the record did not survive the snapshot")
	}
	if !reflect.DeepEqual(rec.Maintainers, protocol.MaintainerList{"@ops"}) {
		t.Errorf("the record lost its reference to the emptied group: %q", rec.Maintainers)
	}
}

// Emptying leaves the group and leaves every record that names it alone. This
// is the half that makes the refusal above affordable: what deletion was for
// is still reachable.
func TestEmptyingAGroupLeavesTheRecordsThatNameIt(t *testing.T) {
	bus, s, token := groupFixture(t)
	if err := bus.SetGroup("admin@h", "@ops", []string{"maint@h"}); err != nil {
		t.Fatal(err)
	}
	if code, body := post(t, s, token, "alice@h", "/register", `{"kind":"agent","name":"svc@h","allow":["alice@h","@ops"]}`); code != 200 {
		t.Fatalf("register answered %d: %s", code, body)
	}
	if code, body := post(t, s, token, "alice@h", "/manage", `{"kind":"agent","name":"svc@h","maintainers":"@ops"}`); code != 200 {
		t.Fatalf("assigning maintainers answered %d: %s", code, body)
	}
	// The member manages it through the group, which is what emptying revokes.
	if code, body := post(t, s, token, "maint@h", "/manage", `{"kind":"agent","name":"svc@h","descr":"through the group"}`); code != 200 {
		t.Fatalf("a member could not manage through the group: %d %s", code, body)
	}

	if code, body := post(t, s, token, "admin@h", "/group", `{"kind":"agent","name":"@ops","members":[]}`); code != 200 {
		t.Fatalf("emptying answered %d: %s", code, body)
	}

	if _, ok := bus.Groups("admin@h")["@ops"]; !ok {
		t.Error("emptying the membership took the group name with it")
	}
	if code, body := post(t, s, token, "maint@h", "/manage", `{"kind":"agent","name":"svc@h","descr":"revoked"}`); code != 403 {
		t.Errorf("a former member still confers management: %d %s", code, body)
	}
	// The record still names the emptied group in both places. Deletion
	// could not have left this true, which is why it was refused above.
	var rec protocol.Record
	req := httptest.NewRequest("GET", "/lookup?name=svc@h", nil)
	req.Header.Set(HeaderToken, token("alice@h"))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if err := json.Unmarshal(w.Body.Bytes(), &rec); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rec.Maintainers, protocol.MaintainerList{"@ops"}) {
		t.Errorf("emptying the group changed the record that named it: %q", rec.Maintainers)
	}
	if rec.Descr != "through the group" {
		t.Errorf("the record lost the edit made through the group: %q", rec.Descr)
	}

	// Putting one back restores what they confer, so emptying is a state and
	// not a one-way door.
	if code, body := post(t, s, token, "admin@h", "/group", `{"kind":"agent","name":"@ops","members":["maint@h"]}`); code != 200 {
		t.Fatalf("refilling answered %d: %s", code, body)
	}
	if code, body := post(t, s, token, "maint@h", "/manage", `{"kind":"agent","name":"svc@h","descr":"restored"}`); code != 200 {
		t.Errorf("refilling the group did not restore management: %d %s", code, body)
	}
}

// @administrators refuses to be emptied, because the daemon owner stays in it.
// Retirement is for ordinary groups; the one group that defines a level is not
// retirable by either route.
func TestTheAdministratorsGroupIsNeitherEmptiedNorRemoved(t *testing.T) {
	_, s, token := groupFixture(t)
	if code, body := post(t, s, token, "admin@h", "/group", `{"kind":"agent","name":"@administrators","members":[]}`); code != 403 {
		t.Errorf("the maintainers group was emptied: %d %s", code, body)
	}
	if code, body := post(t, s, token, "admin@h", "/group", `{"kind":"agent","name":"@administrators","remove":true}`); code != 400 {
		t.Errorf("the maintainers group answered a removal %d, want 400: %s", code, body)
	}
}
