package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The locks API end to end: the Group is namespace and ACL, membership and
// liveness come from the registry, the table from the locks package
// (docs/01-identity-and-roles.md#shared-locks).
func locksFixture(t *testing.T) (*Server, func(string) string, *recorder) {
	t.Helper()
	bus := core.New()
	s, tok := serverFor(t, bus, "admin@h")
	for _, who := range []string{"alice@h", "bob@h", "carol@h"} {
		if _, err := bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	// @ops nests @team: nested membership is checked at take time. @team first:
	// a group member that does not exist yet is a refusal. Alice owns both, so
	// her going inactive is what makes the groups inactive.
	for _, g := range []struct {
		name    string
		members []string
	}{{"@team", []string{"bob@h"}}, {"@ops", []string{"alice@h", "@team"}}} {
		if err := bus.SetGroup("alice@h", g.name, g.members); err != nil {
			t.Fatal(err)
		}
	}
	rec := &recorder{}
	s.Journal(rec)
	return s, tok, rec
}

func take(s *Server, token, path, body string) (int, string) { return send(s, token, "POST", path, body, "") }

func TestLocksEndToEnd(t *testing.T) {
	s, tok, rec := locksFixture(t)
	alice, bob, carol := tok("alice@h"), tok("bob@h"), tok("carol@h")

	// A member takes; the nested-group member too; a non-member is refused.
	code, body := take(s, alice, "/try-lock", `{"group":"@ops","name":"deploy","ttl":"1m"}`)
	if code != http.StatusOK || !strings.Contains(body, `"holder":"alice@h"`) {
		t.Fatalf("member try-lock: %d %s", code, body)
	}
	if code, body = take(s, bob, "/try-lock", `{"group":"@ops","name":"other","ttl":"1m"}`); code != http.StatusOK {
		t.Fatalf("nested member: %d %s", code, body)
	}
	if code, _ = take(s, carol, "/try-lock", `{"group":"@ops","name":"x","ttl":"1m"}`); code != http.StatusForbidden {
		t.Fatalf("a non-member took a lock: %d", code)
	}

	// One holder at a time, the refusal names the holder, and lock waits.
	if code, body = take(s, bob, "/try-lock", `{"group":"@ops","name":"deploy","ttl":"1m"}`); code != http.StatusConflict || !strings.Contains(body, "alice@h") {
		t.Fatalf("second take: %d %s", code, body)
	}
	start := time.Now()
	go func() {
		time.Sleep(300 * time.Millisecond)
		if code, body := take(s, alice, "/release", `{"group":"@ops","name":"deploy"}`); code != http.StatusOK {
			t.Errorf("release: %d %s", code, body)
		}
	}()
	code, body = take(s, bob, "/lock?wait=5s", `{"group":"@ops","name":"deploy","ttl":"1m"}`)
	if code != http.StatusOK || !strings.Contains(body, `"holder":"bob@h"`) {
		t.Fatalf("waiting take: %d %s", code, body)
	}
	if d := time.Since(start); d < 200*time.Millisecond {
		t.Fatalf("the wait returned in %v, without the holder releasing", d)
	}

	// Only the holder releases; --force releases another's and is audited.
	if code, _ = take(s, alice, "/release", `{"group":"@ops","name":"deploy"}`); code != http.StatusConflict {
		t.Fatalf("a non-holder released: %d", code)
	}
	if code, _ := take(s, alice, "/release?force=1", `{"group":"@ops","name":"deploy"}`); code != http.StatusOK {
		t.Fatalf("force release: %d", code)
	}
	var audited bool
	for _, e := range rec.audit {
		if strings.Contains(e.Operation, "release") && e.Actor == "alice@h" {
			audited = true
		}
	}
	if !audited {
		t.Fatal("the force release was not audited for alice")
	}

	// Holders answers any member.
	code, body = send(s, alice, "GET", "/holders?group=@ops", "", "")
	if code != http.StatusOK || !strings.Contains(body, `"other":"bob@h"`) || strings.Contains(body, "deploy") {
		t.Fatalf("holders: %d %s", code, body)
	}

	// A bad ttl is a malformed refusal; an unknown group is no such name.
	if code, _ = take(s, alice, "/try-lock", `{"group":"@ops","name":"x","ttl":"soon"}`); code != http.StatusBadRequest {
		t.Fatalf("a bad ttl was not malformed: %d", code)
	}
	if code, _ = take(s, alice, "/try-lock", `{"group":"@nobody","name":"x","ttl":"1m"}`); code != http.StatusNotFound {
		t.Fatalf("an unknown group was not no-such-name: %d", code)
	}
}

// An inactive group has no locks: taking is refused and what it held is gone.
func TestAnInactiveGroupHasNoLocks(t *testing.T) {
	s, tok, _ := locksFixture(t)
	bus := s.bus
	alice := tok("alice@h")
	if code, _ := take(s, alice, "/try-lock", `{"group":"@ops","name":"deploy","ttl":"1m"}`); code != http.StatusOK {
		t.Fatal("the member could not take the lock first")
	}
	if _, err := bus.SetUserState("admin@h", "alice@h", protocol.StatusInactive); err != nil {
		t.Fatal(err)
	}
	bob := tok("bob@h")
	if code, _ := take(s, bob, "/try-lock", `{"group":"@ops","name":"deploy","ttl":"1m"}`); code != http.StatusNotFound {
		t.Fatalf("an inactive group still took: %d", code)
	}
	// The group is active again; the lock it held is gone with the group's absence.
	if _, err := bus.SetUserState("admin@h", "alice@h", protocol.StatusActive); err != nil {
		t.Fatal(err)
	}
	if code, body := take(s, bob, "/try-lock", `{"group":"@ops","name":"deploy","ttl":"1m"}`); code != http.StatusOK || strings.Contains(body, "alice") {
		t.Fatalf("the hold outlived the group's absence: %d %s", code, body)
	}
}
