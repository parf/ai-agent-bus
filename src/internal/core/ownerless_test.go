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
	b.SetDaemonOwner("owner@h")

	// A service, owned by the owner. Its credential answers for the record.
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "owner@h"}); err != nil {
		t.Fatal(err)
	}
	// A person with a profile and no record of their own — SetUser makes one,
	// so make this one the way a directory restore would.
	b.Restore(ports.Snapshot{
		Users:   []protocol.User{{Name: "person@h", State: "active"}, {Name: "banned@h", State: "banned"}},
		Records: []protocol.Record{userRecord("person@h"), userRecord("banned@h")},
	})

	got := b.Ownerless([]string{
		"owner@h",  // the daemon owner: a profile, written by Administrator
		"#svc@h",    // a record of its own
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
	early.SetDaemonOwner("owner@h")
	if got := early.Ownerless([]string{"person@h", "#svc@h"}); len(got) != 2 {
		t.Errorf("before restore %v is ownerless; the test cannot show the ordering matters", got)
	}
	early.Restore(ports.Snapshot{
		Users:   []protocol.User{{Name: "person@h", State: "active"}},
		Records: []protocol.Record{userRecord("person@h"), {Kind: protocol.KindAgent, Name: "#svc@h", Owner: "owner@h", Full: protocol.OverflowStrict}},
	})
	if got := early.Ownerless([]string{"person@h", "#svc@h"}); len(got) != 0 {
		t.Errorf("after restore %v is still ownerless", got)
	}

	// Before Administrator: nothing has written the owner a profile, and the
	// owner's credential is minted by the store rather than by a record, so a
	// sweep here takes the one credential that must never go.
	cold := New()
	if got := cold.Ownerless([]string{"owner@h"}); len(got) != 1 {
		t.Fatal("the owner is already a user before anybody said so; the check below proves nothing")
	}
	cold.SetDaemonOwner("owner@h")
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
	rep := &reports{}
	b.Journal(rep)
	b.SetDaemonOwner("owner@h")
	// absent@h holds no record and no profile. A record it "owns" is one no
	// version wrote: it is ignored at load and reported, not kept
	// (docs/constitution.md#persistence-and-loading).
	b.Restore(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "#theirs@h", Owner: "absent@h", Kind: protocol.KindAgent, Full: protocol.OverflowStrict},
	}})
	if _, still := b.Lookup("owner@h", "#theirs@h"); still {
		t.Error("a record owned by nobody was loaded")
	}
	if !rep.has("#theirs@h is ignored") {
		t.Errorf("the ignored record was not reported: %v", rep.lines)
	}
	// So the credential behind it answers for nothing and is swept.
	if got := b.Ownerless([]string{"absent@h"}); len(got) != 1 || got[0] != "absent@h" {
		t.Errorf("the credential behind an ignored record survived: %v", got)
	}
	// And a user's own record is kept by the record test, not by this one.
	known(t, b, "self@h")
	if got := b.Ownerless([]string{"self@h"}); len(got) != 0 {
		t.Errorf("a user was swept: %v", got)
	}
}
