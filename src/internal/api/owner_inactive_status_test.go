package api

import (
	"encoding/json"
	"testing"
)

// GET /status answers owner_inactive — records inactive through their owning
// User and the messages they hold — to the daemon Owner and the
// Administrators, and leaves the field out for everybody else
// (docs/05-discovery.md#overview-and-diagnostics).
func TestStatusCountsOwnerInactiveRecordsForAdministratorsOnly(t *testing.T) {
	f := suspendedOwnerFixture(t)
	f.call("bystander@h", "POST", "/send", `{"to":"#svc@h","body":"one"}`, 200)
	f.call("bystander@h", "POST", "/send", `{"to":"#svc@h","body":"two"}`, 200)
	f.call("bystander@h", "POST", "/send", `{"to":"#steady-svc@h","body":"not counted"}`, 200)

	read := func(who string) (map[string]int, bool) {
		t.Helper()
		var st struct {
			OwnerInactive *map[string]int `json:"owner_inactive"`
		}
		if err := json.Unmarshal([]byte(f.call(who, "GET", "/status", "", 200)), &st); err != nil {
			t.Fatal(err)
		}
		if st.OwnerInactive == nil {
			return nil, false
		}
		return *st.OwnerInactive, true
	}
	if got, ok := read("admin@h"); !ok || got["records"] != 0 || got["messages"] != 0 {
		t.Fatalf("before the deactivation the daemon Owner reads %v present=%v, want present zeros", got, ok)
	}
	f.state("alice@h", "inactive")
	for _, who := range []string{"admin@h", "maint@h"} {
		if got, ok := read(who); !ok || got["records"] != 1 || got["messages"] != 2 {
			t.Errorf("%s reads %v present=%v, want 1 record holding 2 messages", who, got, ok)
		}
	}
	for _, who := range []string{"bystander@h", "steady@h"} {
		if got, ok := read(who); ok {
			t.Errorf("%s, no Administrator, is answered the node-wide count %v", who, got)
		}
	}
}
