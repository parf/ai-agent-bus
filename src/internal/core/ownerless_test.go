package core

import (
	"slices"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A credential belongs to nobody when its name has no record of its own and no
// registered user. Both tests, and the daemon's own — never how a name is
// spelled. See docs/02-access.md#ownerless-credentials.
//
// This node had 232 such names against 4 records: launcher sessions and test
// fixtures whose records were long gone, every one of them listed as a user.
func TestOwnerlessIsNoRecordAndNoUser(t *testing.T) {
	b := New()
	b.Administrator("owner@h")

	// A service, owned by the owner. Its credential answers for the record.
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "owner@h"}); err != nil {
		t.Fatal(err)
	}
	// A person with a profile and no record of their own — SetUser makes one,
	// so make this one the way a directory restore would.
	b.Restore(ports.Snapshot{Users: []protocol.User{
		{Name: "person@h", State: "active"},
		{Name: "banned@h", State: "banned"},
	}})

	got := b.Ownerless([]string{
		"owner@h",  // the daemon owner: a profile, written by Administrator
		"svc@h",    // a record of its own
		"person@h", // a registered user
		"banned@h", // a banned one is still registered
		"junk@h",   // neither
		"other@h",  // neither
	})
	want := []string{"junk@h", "other@h"}
	if !slices.Equal(got, want) {
		t.Errorf("ownerless is %v, want %v", got, want)
	}
}

// The order the daemon does it in is the whole of the correctness. Sweeping
// before the snapshot is restored takes every credential on the node, and
// sweeping before the administrator is set takes the owner's own.
func TestTheSweepNeedsRestoreAndTheOwnerFirst(t *testing.T) {
	// Before Restore: the records and users are not there yet, so everything
	// reads as ownerless.
	early := New()
	early.Administrator("owner@h")
	if got := early.Ownerless([]string{"person@h", "svc@h"}); len(got) != 2 {
		t.Errorf("before restore %v is ownerless; the test cannot show the ordering matters", got)
	}
	early.Restore(ports.Snapshot{
		Users:   []protocol.User{{Name: "person@h", State: "active"}},
		Records: []protocol.Record{{Name: "svc@h", Owner: "owner@h"}},
	})
	if got := early.Ownerless([]string{"person@h", "svc@h"}); len(got) != 0 {
		t.Errorf("after restore %v is still ownerless", got)
	}

	// Before Administrator: nothing has written the owner a profile, and the
	// owner's credential is minted by the store rather than by a record, so a
	// sweep here takes the one credential that must never go.
	cold := New()
	if got := cold.Ownerless([]string{"owner@h"}); len(got) != 1 {
		t.Fatal("the owner is already a user before anybody said so; the check below proves nothing")
	}
	cold.Administrator("owner@h")
	if got := cold.Ownerless([]string{"owner@h"}); len(got) != 0 {
		t.Errorf("the daemon owner's own credential is ownerless: %v", got)
	}
}

// The replacement the interim guard's own comment asked for. That guard spared
// a credential whose name still owned services, because taking it would have
// manufactured the very orphans the rule exists to clean up. Orphan deletion
// exists now, so the guard is gone and the two sweeps have to agree about one
// name in one run: the records go first, and the credential goes because they
// went. See docs/02-access.md#ownerless-credentials.
func TestTheTwoSweepsAgreeAboutOneName(t *testing.T) {
	b := New()
	b.Administrator("owner@h")
	// absent@h holds no record and no profile, and owns a service. Restored
	// rather than registered: registering a record owned by a name the daemon
	// knows nothing about is refused now
	// (docs/01-identity.md#when-the-owner-is-gone), so an old store is the
	// only place this state still comes from \u2014 which is the state the sweeps
	// have to be safe against.
	b.Restore(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "theirs@h", Owner: "absent@h", Kind: "generic", Full: protocol.OverflowStrict},
	}})

	// Order is the whole of it. Before the records go, the credential is
	// behind something and taking it would strand theirs@h.
	if got := b.Ownerless([]string{"absent@h"}); len(got) != 1 {
		t.Errorf("a name the daemon holds nothing for is not swept: %v", got)
	}
	if purged := b.Orphans(); len(purged) != 1 || purged[0] != "theirs@h" {
		t.Fatalf("the orphan was not deleted: %v", purged)
	}
	if _, still := b.Lookup("owner@h", "theirs@h"); still {
		t.Error("the orphan is still in the registry")
	}
	// And now nothing is stranded by taking it, which is what makes the
	// guard unnecessary rather than merely removed.
	if got := b.Ownerless([]string{"absent@h"}); len(got) != 1 || got[0] != "absent@h" {
		t.Errorf("the credential behind a deleted orphan survived: %v", got)
	}

	// Positive control: the same name with nothing of its own is swept, so the
	// check above is about owning services and not about the name.
	if got := b.Ownerless([]string{"nobody@h"}); len(got) != 1 {
		t.Errorf("a name owning nothing is not swept either: %v", got)
	}
	// And a self-owned record is kept by the record test, not by this one.
	known(t, b, "self@h")
	if got := b.Ownerless([]string{"self@h"}); len(got) != 0 {
		t.Errorf("a self-owned record was swept: %v", got)
	}
}
