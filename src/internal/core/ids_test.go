package core

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func recordID(t *testing.T, b *Bus, name string) uint32 {
	t.Helper()
	b.mu.Lock()
	defer b.unlock()
	r, ok := b.records[name]
	if !ok {
		t.Fatalf("no record %s", name)
	}
	return r.ID
}

// IDs are stable across a restart, and a removed record's ID is never handed
// out again — not in the same run, and not after a restart that no longer has
// the removed record to look at (docs/constitution.md#common-record-fields).
func TestRecordIDsAreStableAndNeverReused(t *testing.T) {
	d := memory.NewState()
	b := durabilityFixture(t, d)
	svc := recordID(t, b, "#svc@h")
	if svc == 0 {
		t.Fatal("a stored record has no ID")
	}
	descr := "edited"
	if _, err := b.Manage("alice@h", Management{Name: "#svc@h", Descr: &descr}); err != nil {
		t.Fatal(err)
	}
	if recordID(t, b, "#svc@h") != svc {
		t.Fatal("an edit changed the record's ID")
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#last@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	last := recordID(t, b, "#last@h")
	if err := b.Unregister("#last@h", "alice@h"); err != nil {
		t.Fatal(err)
	}
	recovered := recoverFrom(t, d)
	if recordID(t, recovered, "#svc@h") != svc {
		t.Fatal("a restart changed the record's ID")
	}
	if _, err := recovered.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#next@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	if next := recordID(t, recovered, "#next@h"); next <= last {
		t.Fatalf("a record created after a restart reused ID %d (the removed record had %d)", next, last)
	}
}

func TestUserIDsAreStable(t *testing.T) {
	d := memory.NewState()
	b := durabilityFixture(t, d)
	b.mu.Lock()
	bob := b.users["bob@h"].ID
	b.unlock()
	if bob == 0 {
		t.Fatal("a stored user has no ID")
	}
	if _, err := b.EditOwnEmail("bob@h", "bob@example.com"); err != nil {
		t.Fatal(err)
	}
	recovered := recoverFrom(t, d)
	if name, ok := recovered.UserName(bob); !ok || name != "bob@h" {
		t.Fatalf("user ID %d answers %q after a restart", bob, name)
	}
}

// The ID index is rebuilt from what was loaded.
func TestIDIndexIsRebuiltOnLoad(t *testing.T) {
	d := memory.NewState()
	b := durabilityFixture(t, d)
	svc := recordID(t, b, "#svc@h")
	recovered := recoverFrom(t, d)
	if name, ok := recovered.RecordName(svc); !ok || name != "#svc@h" {
		t.Fatalf("ID %d answers %q, %v after a restart", svc, name, ok)
	}
}

// Past the last ID there are no more: a new entity is refused rather than
// wrapped onto an ID in use, and nothing of it is kept.
func TestIDExhaustionRefusesRatherThanWraps(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	b.Restore(ports.Snapshot{NextRecordID: ^uint32(0)})
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#late@h", Owner: "admin@h"}); !errors.Is(err, ErrExhausted) {
		t.Fatalf("got %v", err)
	}
	if _, ok := b.Lookup("admin@h", "#late@h"); ok {
		t.Fatal("a refused registration was kept")
	}
}

// An internal ID is on no answer.
func TestIDsAreNotPublic(t *testing.T) {
	b := durabilityFixture(t, nil)
	r, ok := b.Lookup("alice@h", "#svc@h")
	if !ok || r.ID == 0 {
		t.Fatal("fixture")
	}
	out, _ := json.Marshal(r)
	if strings.Contains(string(out), `"id"`) || strings.Contains(string(out), `"ID"`) {
		t.Fatalf("an internal ID left the daemon: %s", out)
	}
}
