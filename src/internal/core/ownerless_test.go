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

// Interim, and named so it is found when H.5.5 lands: a name that owns
// services but holds no record of its own keeps its credential, because taking
// it would leave every one of those services with an owner nothing answers
// for — the orphans that rule exists to clean up, manufactured at a restart,
// silently. When orphan-service deletion exists this guard is pointless and
// this check should be replaced, not deleted.
// See docs/02-access.md#ownerless-credentials.
func TestTheSweepDoesNotManufactureOrphans(t *testing.T) {
	b := New()
	b.Administrator("owner@h")
	// absent@h holds no record and no profile, and owns a service.
	if _, err := b.Register(protocol.Record{Name: "theirs@h", Owner: "absent@h"}); err != nil {
		t.Fatal(err)
	}
	if got := b.Ownerless([]string{"absent@h"}); len(got) != 0 {
		t.Errorf("%v swept, leaving theirs@h owned by a name with no credential", got)
	}
	// Positive control: the same name with nothing of its own is swept, so the
	// check above is about owning services and not about the name.
	if got := b.Ownerless([]string{"nobody@h"}); len(got) != 1 {
		t.Errorf("a name owning nothing is not swept either: %v", got)
	}
	// And a self-owned record is kept by the record test, not by this one.
	if _, err := b.Register(protocol.Record{Name: "self@h", Owner: "self@h"}); err != nil {
		t.Fatal(err)
	}
	if got := b.Ownerless([]string{"self@h"}); len(got) != 0 {
		t.Errorf("a self-owned record was swept: %v", got)
	}
}
