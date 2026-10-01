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

// The locks API end to end: the record is the namespace, and its Owner and
// Maintainers — nested groups included — may use its locks; management and
// liveness come from the registry, the table from the locks package
// (docs/01-identity-and-authority.md#shared-locks).
func locksFixture(t *testing.T) (*Server, func(string) string, *recorder) {
	t.Helper()
	bus := core.New()
	s, tok := serverFor(t, bus, "admin@h")
	for _, who := range []string{"alice@h", "bob@h", "carol@h"} {
		if _, err := bus.SetUser("admin@h", protocol.User{Name: who}, true); err != nil {
			t.Fatal(err)
		}
	}
	// ops@h is alice's; @team (bob) maintains it, so a Maintainer through a
	// group is checked at take time. carol is on its allow list, which grants
	// access and not management: she may not use its locks. Alice owns it, so
	// her going inactive is what makes the record inactive.
	if err := bus.SetGroup("alice@h", "@team", []string{"bob@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := bus.Register(protocol.Record{Name: "ops@h", Kind: protocol.KindQueue, Owner: "alice@h",
		Maintainers: protocol.MaintainerList{"@team"}, Allow: []string{"carol@h"}}); err != nil {
		t.Fatal(err)
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
	code, body := take(s, alice, "/try-lock", `{"record":"ops@h","name":"deploy","ttl":"1m"}`)
	if code != http.StatusOK || !strings.Contains(body, `"holder":"alice@h"`) {
		t.Fatalf("member try-lock: %d %s", code, body)
	}
	if code, body = take(s, bob, "/try-lock", `{"record":"ops@h","name":"other","ttl":"1m"}`); code != http.StatusOK {
		t.Fatalf("nested member: %d %s", code, body)
	}
	if code, _ = take(s, carol, "/try-lock", `{"record":"ops@h","name":"x","ttl":"1m"}`); code != http.StatusForbidden {
		t.Fatalf("a caller on the allow list, not a Maintainer, took a lock: %d", code)
	}

	// One holder at a time, the refusal names the holder, and lock waits.
	if code, body = take(s, bob, "/try-lock", `{"record":"ops@h","name":"deploy","ttl":"1m"}`); code != http.StatusConflict || !strings.Contains(body, "alice@h") {
		t.Fatalf("second take: %d %s", code, body)
	}
	start := time.Now()
	go func() {
		time.Sleep(300 * time.Millisecond)
		if code, body := take(s, alice, "/release", `{"record":"ops@h","name":"deploy"}`); code != http.StatusOK {
			t.Errorf("release: %d %s", code, body)
		}
	}()
	code, body = take(s, bob, "/lock?wait=5s", `{"record":"ops@h","name":"deploy","ttl":"1m"}`)
	if code != http.StatusOK || !strings.Contains(body, `"holder":"bob@h"`) {
		t.Fatalf("waiting take: %d %s", code, body)
	}
	if d := time.Since(start); d < 200*time.Millisecond {
		t.Fatalf("the wait returned in %v, without the holder releasing", d)
	}

	// Only the holder releases; --force releases another's and is audited.
	if code, _ = take(s, alice, "/release", `{"record":"ops@h","name":"deploy"}`); code != http.StatusConflict {
		t.Fatalf("a non-holder released: %d", code)
	}
	if code, body := take(s, alice, "/release-force", `{"record":"ops@h","name":"deploy"}`); code != http.StatusOK || !strings.Contains(body, "bob@h") {
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
	code, body = send(s, alice, "GET", "/holders?record=ops@h", "", "")
	if code != http.StatusOK || !strings.Contains(body, `"other":{"holder":"bob@h"`) || strings.Contains(body, "deploy") {
		t.Fatalf("holders: %d %s", code, body)
	}

	// A bad ttl is a malformed refusal; an unknown group is no such name.
	if code, _ = take(s, alice, "/try-lock", `{"record":"ops@h","name":"x","ttl":"soon"}`); code != http.StatusBadRequest {
		t.Fatalf("a bad ttl was not malformed: %d", code)
	}
	if code, _ = take(s, alice, "/try-lock", `{"record":"nobody@h","name":"x","ttl":"1m"}`); code != http.StatusNotFound {
		t.Fatalf("an unknown group was not no-such-name: %d", code)
	}
}

// An inactive group has no locks: taking is refused and what it held is gone.
func TestAnInactiveGroupHasNoLocks(t *testing.T) {
	s, tok, _ := locksFixture(t)
	bus := s.bus
	alice := tok("alice@h")
	if code, _ := take(s, alice, "/try-lock", `{"record":"ops@h","name":"deploy","ttl":"1m"}`); code != http.StatusOK {
		t.Fatal("the member could not take the lock first")
	}
	if code, body := send(s, tok("admin@h"), "POST", "/user/state", `{"name":"alice@h","status":"inactive"}`, ""); code != http.StatusOK {
		t.Fatalf("deactivation: %d %s", code, body)
	}
	bob := tok("bob@h")
	if code, _ := take(s, bob, "/try-lock", `{"record":"ops@h","name":"deploy","ttl":"1m"}`); code != http.StatusNotFound {
		t.Fatalf("an inactive group still took: %d", code)
	}
	// The group is active again; the lock it held is gone with the group's absence.
	if _, err := bus.SetUserState("admin@h", "alice@h", protocol.StatusActive); err != nil {
		t.Fatal(err)
	}
	if code, body := take(s, bob, "/try-lock", `{"record":"ops@h","name":"deploy","ttl":"1m"}`); code != http.StatusOK || strings.Contains(body, "alice") {
		t.Fatalf("the hold outlived the group's absence: %d %s", code, body)
	}
}

// The gate covers every verb: a non-member's release, force-release and
// holders call are refused; an inactive group's too; extend and self-take
// answer their own refusals; and the clamps are malformed refusals.
func TestLockGatesAndClamps(t *testing.T) {
	s, tok, _ := locksFixture(t)
	alice, carol := tok("alice@h"), tok("carol@h")
	s.locks.Take(context.Background(), "ops@h", "db", "alice@h", time.Minute, 0)

	for _, call := range []struct{ path, body string }{
		{"/release", `{"record":"ops@h","name":"db"}`},
		{"/release-force", `{"record":"ops@h","name":"db"}`},
	} {
		if code, _ := take(s, carol, call.path, call.body); code != http.StatusForbidden {
			t.Fatalf("a non-member's %s: %d, want 403", call.path, code)
		}
	}
	if code, _ := send(s, carol, "GET", "/holders?record=ops@h", "", ""); code != http.StatusForbidden {
		t.Fatalf("a non-member's holders: %d, want 403", code)
	}
	// An inactive group refuses the same calls as no such name.
	if code, body := send(s, tok("admin@h"), "POST", "/user/state", `{"name":"alice@h","status":"inactive"}`, ""); code != http.StatusOK {
		t.Fatalf("deactivation: %d %s", code, body)
	}
	for _, call := range []struct{ path, body string }{
		{"/release", `{"record":"ops@h","name":"db"}`},
		{"/release-force", `{"record":"ops@h","name":"db"}`},
	} {
		if code, _ := take(s, tok("bob@h"), call.path, call.body); code != http.StatusNotFound {
			t.Fatalf("an inactive group's %s: %d, want 404", call.path, code)
		}
	}
	if code, _ := send(s, tok("bob@h"), "GET", "/holders?record=ops@h", "", ""); code != http.StatusNotFound {
		t.Fatalf("an inactive group's holders: %d, want 404", code)
	}
	if _, err := s.bus.SetUserState("admin@h", "alice@h", "active"); err != nil {
		t.Fatal(err)
	}
	if code, _ := take(s, alice, "/try-lock", `{"record":"ops@h","name":"db","ttl":"1m"}`); code != http.StatusOK {
		t.Fatalf("re-taking after the group's return: %d", code)
	}

	// Extend: holder only, fresh ttl; self-take is refused at once.
	bob := tok("bob@h")
	if code, body := take(s, bob, "/extend", `{"record":"ops@h","name":"db","ttl":"1m"}`); code != http.StatusConflict || !strings.Contains(body, "alice@h") {
		t.Fatalf("a non-holder extended: %d %s", code, body)
	}
	if code, _ := take(s, alice, "/extend", `{"record":"ops@h","name":"db","ttl":"1m"}`); code != http.StatusOK {
		t.Fatalf("the holder could not extend: %d", code)
	}
	if code, body := take(s, alice, "/try-lock", `{"record":"ops@h","name":"db","ttl":"1m"}`); code != http.StatusConflict || !strings.Contains(body, "already hold it") {
		t.Fatalf("self-take: %d %s", code, body)
	}
	// A member's waiting lock on their own hold is refused at once too.
	if code, body := take(s, alice, "/lock?wait=5s", `{"record":"ops@h","name":"db","ttl":"1m"}`); code != http.StatusConflict || !strings.Contains(body, "already hold it") {
		t.Fatalf("self-take with a wait: %d %s", code, body)
	}
	// A held lock nobody holds any more.
	if code, _ := take(s, alice, "/extend", `{"record":"ops@h","name":"none","ttl":"1m"}`); code != http.StatusNotFound {
		t.Fatalf("extending what nobody holds: %d", code)
	}
	// The clamps.
	if code, _ := take(s, alice, "/try-lock", `{"record":"ops@h","name":"x","ttl":"25h"}`); code != http.StatusBadRequest {
		t.Fatalf("a 25h ttl was not refused: %d", code)
	}
	if code, _ := take(s, alice, "/lock?wait=61s", `{"record":"ops@h","name":"x","ttl":"1m"}`); code != http.StatusBadRequest {
		t.Fatalf("a 61s wait was not refused: %d", code)
	}
}

// A deactivation through the API ends every lock now, not at the next lazy
// call: reactivate the group's owner and the hold is gone.
func TestDeactivationEndsLocks(t *testing.T) {
	s, tok, _ := locksFixture(t)
	alice := tok("alice@h")
	if code, _ := take(s, alice, "/try-lock", `{"record":"ops@h","name":"deploy","ttl":"10m"}`); code != http.StatusOK {
		t.Fatal("the member could not take the lock first")
	}
	// Deactivate alice: ops@h (hers) goes inactive, and the reset fires on the
	// state change itself.
	if code, body := send(s, tok("admin@h"), "POST", "/user/state", `{"name":"alice@h","status":"inactive"}`, ""); code != http.StatusOK {
		t.Fatalf("deactivation: %d %s", code, body)
	}
	if _, err := s.bus.SetUserState("admin@h", "alice@h", "active"); err != nil {
		t.Fatal(err)
	}
	if got := s.locks.Holders("ops@h"); len(got) != 0 {
		t.Fatalf("the hold outlived the deactivation: %v", got)
	}
	// The group is back and the lock is free.
	if code, _ := take(s, tok("bob@h"), "/try-lock", `{"record":"ops@h","name":"deploy","ttl":"10m"}`); code != http.StatusOK {
		t.Fatal("the lock stayed wedged after the reactivation")
	}
}

// Deactivating a Group through /manage ends its locks: the manage route's
// reset is its own, not a side effect of the user-state one.
func TestManageDeactivationEndsLocks(t *testing.T) {
	s, tok, _ := locksFixture(t)
	alice := tok("alice@h")
	if code, _ := take(s, alice, "/try-lock", `{"record":"ops@h","name":"deploy","ttl":"10m"}`); code != http.StatusOK {
		t.Fatal("the member could not take the lock first")
	}
	// ops@h is alice's; deactivating the record itself through /manage.
	if code, body := send(s, alice, "POST", "/manage", `{"name":"ops@h","status":"inactive"}`, ""); code != http.StatusOK {
		t.Fatalf("manage deactivation: %d %s", code, body)
	}
	// The group is back; the lock it held is gone.
	if _, err := s.bus.Manage("alice@h", core.Management{Name: "ops@h", Status: &[]string{"active"}[0]}); err != nil {
		t.Fatal(err)
	}
	if got := s.locks.Holders("ops@h"); len(got) != 0 {
		t.Fatalf("the hold outlived the group's deactivation through /manage: %v", got)
	}
	if code, _ := take(s, tok("bob@h"), "/try-lock", `{"record":"ops@h","name":"deploy","ttl":"10m"}`); code != http.StatusOK {
		t.Fatal("the lock stayed wedged after the group's return")
	}
}

// Deactivating one record ends its locks and nobody else's.
func TestDeactivatingOneRecordLeavesOtherLocks(t *testing.T) {
	s, tok, _ := locksFixture(t)
	alice := tok("alice@h")
	if _, err := s.bus.Register(protocol.Record{Name: "old@h", Kind: protocol.KindQueue, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	for _, rec := range []string{"ops@h", "old@h"} {
		if code, _ := take(s, alice, "/try-lock", `{"record":"`+rec+`","name":"deploy","ttl":"10m"}`); code != http.StatusOK {
			t.Fatalf("could not take %s's lock", rec)
		}
	}
	if code, body := send(s, alice, "POST", "/manage", `{"name":"old@h","status":"inactive"}`, ""); code != http.StatusOK {
		t.Fatalf("deactivate: %d %s", code, body)
	}
	if got := s.locks.Holders("ops@h"); got["deploy"].Holder != "alice@h" {
		t.Fatalf("an unrelated record's lock went with the deactivation: %v", got)
	}
	if got := s.locks.Holders("old@h"); len(got) != 0 {
		t.Fatalf("the deactivated record kept its lock: %v", got)
	}
}

// /holders with no record lists every lock the caller may use, and only those.
func TestHoldersWithNoRecordListsWhatTheCallerMayUse(t *testing.T) {
	s, tok, _ := locksFixture(t)
	alice, bob, carol := tok("alice@h"), tok("bob@h"), tok("carol@h")
	if _, err := s.bus.Register(protocol.Record{Name: "mine@h", Kind: protocol.KindQueue, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	for _, rec := range []string{"ops@h", "mine@h"} {
		if code, _ := take(s, alice, "/try-lock", `{"record":"`+rec+`","name":"deploy","ttl":"10m"}`); code != http.StatusOK {
			t.Fatalf("could not take %s's lock", rec)
		}
	}
	// bob maintains ops@h through @team, and has nothing to do with mine@h.
	code, body := send(s, bob, "GET", "/holders", "", "")
	if code != http.StatusOK || !strings.Contains(body, `"record":"ops@h","kind":"queue","name":"deploy","holder":"alice@h"`) || strings.Contains(body, "mine@h") {
		t.Fatalf("bob's listing: %d %s", code, body)
	}
	// carol is on ops@h's allow list only: use of the record, not its locks.
	if code, body = send(s, carol, "GET", "/holders", "", ""); code != http.StatusOK || strings.TrimSpace(body) != "[]" {
		t.Fatalf("carol's listing: %d %s", code, body)
	}
	if code, body = send(s, alice, "GET", "/holders", "", ""); code != http.StatusOK || strings.Count(body, `"name":"deploy"`) != 2 {
		t.Fatalf("alice's listing: %d %s", code, body)
	}
	// Sorted by record: six of them, so a map's own order is no pass.
	names := []string{"f@h", "b@h", "e@h", "a@h", "d@h", "c@h"}
	for _, n := range names {
		if _, err := s.bus.Register(protocol.Record{Name: n, Kind: protocol.KindQueue, Owner: "alice@h"}); err != nil {
			t.Fatal(err)
		}
		if code, _ := take(s, alice, "/try-lock", `{"record":"`+n+`","name":"x","ttl":"10m"}`); code != http.StatusOK {
			t.Fatalf("could not take %s", n)
		}
	}
	_, body = send(s, alice, "GET", "/holders", "", "")
	for i, n := range []string{"a@h", "b@h", "c@h", "d@h", "e@h", "f@h"}[1:] {
		prev := []string{"a@h", "b@h", "c@h", "d@h", "e@h", "f@h"}[i]
		if strings.Index(body, `"record":"`+prev+`"`) > strings.Index(body, `"record":"`+n+`"`) {
			t.Fatalf("the listing is not sorted by record: %s", body)
		}
	}
}

// Unregister removes every lock on that record, not another record's locks.
// A refused removal must keep the holds, and a reused name starts free.
func TestUnregisterDeletesEveryRecordLock(t *testing.T) {
	s, tok, _ := locksFixture(t)
	alice, carol := tok("alice@h"), tok("carol@h")
	if _, err := s.bus.Register(protocol.Record{Name: "other@h", Kind: protocol.KindQueue, Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	for _, rec := range []string{"ops@h", "other@h"} {
		for _, name := range []string{"deploy", "db"} {
			if code, body := take(s, alice, "/try-lock", `{"record":"`+rec+`","name":"`+name+`","ttl":"10m"}`); code != http.StatusOK {
				t.Fatalf("take: %d %s", code, body)
			}
		}
	}
	if code, body := send(s, carol, "POST", "/unregister", `{"name":"ops@h"}`, ""); code != http.StatusForbidden {
		t.Fatalf("refused unregister: %d %s", code, body)
	}
	if got := s.locks.Holders("ops@h"); len(got) != 2 {
		t.Fatalf("refused unregister deleted locks: %v", got)
	}
	if code, body := send(s, alice, "POST", "/unregister", `{"name":" OPS@H "}`, ""); code != http.StatusOK {
		t.Fatalf("unregister: %d %s", code, body)
	}
	if got := s.locks.Holders("ops@h"); len(got) != 0 {
		t.Fatalf("unregister left locks: %v", got)
	}
	if got := s.locks.Holders("other@h"); len(got) != 2 {
		t.Fatalf("unregister deleted unrelated locks: %v", got)
	}
	if _, err := s.bus.Register(protocol.Record{Name: "ops@h", Kind: protocol.KindQueue, Owner: "carol@h"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"deploy", "db"} {
		if code, body := take(s, carol, "/try-lock", `{"record":"ops@h","name":"`+name+`","ttl":"10m"}`); code != http.StatusOK {
			t.Fatalf("recreated record inherited a lock: %d %s", code, body)
		}
	}
}

// A queued take belongs to its record's current life. Ending that life must
// refuse it rather than grant it when cleanup wakes it.
func TestRecordLifecycleRefusesWaitingLocks(t *testing.T) {
	for _, end := range []struct{ name, path, body, actor string }{
		{"manage", "/manage", `{"name":"ops@h","status":"inactive"}`, "alice@h"},
		{"owner", "/user/state", `{"name":"alice@h","status":"inactive"}`, "admin@h"},
		{"unregister", "/unregister", `{"name":"ops@h"}`, "alice@h"},
	} {
		t.Run(end.name, func(t *testing.T) {
			s, tok, _ := locksFixture(t)
			alice, bob, actor := tok("alice@h"), tok("bob@h"), tok(end.actor)
			if code, body := take(s, alice, "/try-lock", `{"record":"ops@h","name":"deploy","ttl":"10m"}`); code != http.StatusOK {
				t.Fatalf("take: %d %s", code, body)
			}
			done := make(chan int, 1)
			go func() {
				code, _ := take(s, bob, "/lock?wait=2s", `{"record":"ops@h","name":"deploy","ttl":"10m"}`)
				done <- code
			}()
			select {
			case code := <-done:
				t.Fatalf("request did not wait: %d", code)
			case <-time.After(100 * time.Millisecond):
			}
			if code, body := send(s, actor, "POST", end.path, end.body, ""); code != http.StatusOK {
				t.Fatalf("end record: %d %s", code, body)
			}
			select {
			case code := <-done:
				if code != http.StatusNotFound {
					t.Fatalf("waiting take returned %d after %s", code, end.name)
				}
			case <-time.After(time.Second):
				t.Fatal("record cleanup did not wake its waiter")
			}
			if got := s.locks.Holders("ops@h"); len(got) != 0 {
				t.Fatalf("waiting take recreated a hold: %v", got)
			}
		})
	}
}
