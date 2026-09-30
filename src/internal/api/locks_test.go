package api

import (
	"context"
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

func take(s *Server, token, path, body string) (int, string) {
	return send(s, token, "POST", path, body, "")
}

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
	if code, body := take(s, alice, "/release-force", `{"group":"@ops","name":"deploy"}`); code != http.StatusOK || !strings.Contains(body, "bob@h") {
		t.Fatalf("force release: %d %s", code, body)
		t.Fatalf("force release: %d", code)
	}
	var forced bool
	for _, e := range rec.audit {
		if e.Operation == "release --force" && e.Actor == "alice@h" {
			forced = true
		}
	}
	if !forced {
		t.Fatal("the force release was not audited as its own operation for alice")
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

// The gate covers every verb: a non-member's release, force-release and
// holders call are refused; an inactive group's too; extend and self-take
// answer their own refusals; and the clamps are malformed refusals.
func TestLockGatesAndClamps(t *testing.T) {
	s, tok, _ := locksFixture(t)
	alice, carol := tok("alice@h"), tok("carol@h")
	s.locks.Take(context.Background(), "@ops", "db", "alice@h", time.Minute, 0)

	for _, call := range []struct{ path, body string }{
		{"/release", `{"group":"@ops","name":"db"}`},
		{"/release-force", `{"group":"@ops","name":"db"}`},
	} {
		if code, _ := take(s, carol, call.path, call.body); code != http.StatusForbidden {
			t.Fatalf("a non-member's %s: %d, want 403", call.path, code)
		}
	}
	if code, _ := send(s, carol, "GET", "/holders?group=@ops", "", ""); code != http.StatusForbidden {
		t.Fatalf("a non-member's holders: %d, want 403", code)
	}
	// An inactive group refuses the same calls as no such name.
	if _, err := s.bus.SetUserState("admin@h", "alice@h", "inactive"); err != nil {
		t.Fatal(err)
	}
	for _, call := range []struct{ path, body string }{
		{"/release", `{"group":"@ops","name":"db"}`},
		{"/release-force", `{"group":"@ops","name":"db"}`},
	} {
		if code, _ := take(s, tok("bob@h"), call.path, call.body); code != http.StatusNotFound {
			t.Fatalf("an inactive group's %s: %d, want 404", call.path, code)
		}
	}
	if code, _ := send(s, tok("bob@h"), "GET", "/holders?group=@ops", "", ""); code != http.StatusNotFound {
		t.Fatalf("an inactive group's holders: %d, want 404", code)
	}
	if _, err := s.bus.SetUserState("admin@h", "alice@h", "active"); err != nil {
		t.Fatal(err)
	}
	if code, _ := take(s, alice, "/try-lock", `{"group":"@ops","name":"db","ttl":"1m"}`); code != http.StatusOK {
		t.Fatalf("re-taking after the group's return: %d", code)
	}

	// Extend: holder only, fresh ttl; self-take is refused at once.
	bob := tok("bob@h")
	if code, body := take(s, bob, "/extend", `{"group":"@ops","name":"db","ttl":"1m"}`); code != http.StatusConflict || !strings.Contains(body, "alice@h") {
		t.Fatalf("a non-holder extended: %d %s", code, body)
	}
	if code, _ := take(s, alice, "/extend", `{"group":"@ops","name":"db","ttl":"1m"}`); code != http.StatusOK {
		t.Fatalf("the holder could not extend: %d", code)
	}
	if code, body := take(s, alice, "/try-lock", `{"group":"@ops","name":"db","ttl":"1m"}`); code != http.StatusConflict || !strings.Contains(body, "already hold it") {
		t.Fatalf("self-take: %d %s", code, body)
	}
	// A member's waiting lock on their own hold is refused at once too.
	if code, body := take(s, alice, "/lock?wait=5s", `{"group":"@ops","name":"db","ttl":"1m"}`); code != http.StatusConflict || !strings.Contains(body, "already hold it") {
		t.Fatalf("self-take with a wait: %d %s", code, body)
	}
	// A held lock nobody holds any more.
	if code, _ := take(s, alice, "/extend", `{"group":"@ops","name":"none","ttl":"1m"}`); code != http.StatusNotFound {
		t.Fatalf("extending what nobody holds: %d", code)
	}
	// The clamps.
	if code, _ := take(s, alice, "/try-lock", `{"group":"@ops","name":"x","ttl":"25h"}`); code != http.StatusBadRequest {
		t.Fatalf("a 25h ttl was not refused: %d", code)
	}
	if code, _ := take(s, alice, "/lock?wait=61s", `{"group":"@ops","name":"x","ttl":"1m"}`); code != http.StatusBadRequest {
		t.Fatalf("a 61s wait was not refused: %d", code)
	}
}
