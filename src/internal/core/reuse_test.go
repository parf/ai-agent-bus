package core

import (
	"errors"
	"slices"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A name freed by a record ignored at start inherits nothing: registered by
// somebody else, it is on no allow list, in no Group and maintains nothing
// that named the ignored record (K.23, docs/constitution.md#persistence-and-loading).
// A name nobody held keeps what names it in advance.
func TestANameFreedByAnIgnoredRecordInheritsNothing(t *testing.T) {
	b := New()
	rep := &reports{}
	b.Journal(rep)
	assertRestoreRefused(t, b, canonicalFixture(ports.Snapshot{
		Users: []protocol.User{{Name: "alice@h", Status: "active"}, {Name: "eve@h", Status: "active"}},
		Records: []protocol.Record{
			userRecord("alice@h"), userRecord("eve@h"),
			// Owned by nobody, so ignored at load.
			{Name: "#lost@h", Owner: "ghost@h", Kind: protocol.KindAgent},
			{Name: "jobs@h", Owner: "alice@h", Kind: protocol.KindQueue, Allow: []string{"#lost@h", "#future@h"},
				Maintainers: protocol.MaintainerList{"#lost@h"}},
			{Name: "@crew@h", Owner: "alice@h", Kind: protocol.KindGroup, Allow: []string{"#lost@h", "eve@h"}},
		},
	}, ""))
}

// A User ignored at start for lacking its own record keeps its credential for
// the operator who repairs it: the start's ownerless sweep passes it by (K.26).
func TestAnIgnoredUserKeepsItsCredential(t *testing.T) {
	b := New()
	rep := &reports{}
	b.Journal(rep)
	assertRestoreRefused(t, b, canonicalFixture(ports.Snapshot{
		Users:   []protocol.User{{Name: "alice@h", Status: "active"}, {Name: "bob@h", Status: "active"}},
		Records: []protocol.Record{userRecord("alice@h")},
	}, ""))
}

// A User inside a Group on a 📣's deliver_to is skipped at publication with an
// error-log warning, is no failed recipient and adds no drop (K.30, Q126).
func TestAUserInADeliverToGroupIsSkippedAndSaid(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	rep := &reports{}
	b.Journal(rep)
	known(t, b, "alice@h", "bob@h", "#worker@h")
	if err := b.SetGroup("alice@h", "@team@h", []string{"bob@h", "#worker@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "news@h", Kind: protocol.KindPubSub, Owner: "alice@h", Allow: []string{"*"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "news@h", Subs: ptr([]string{"@team@h"})}); err != nil {
		t.Fatal(err)
	}
	ownedBy(t, b, "alice@h", "#worker@h")
	// bob holds no roles toward news@h. The channel Owner manages its recipient.
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "news@h", Body: "hi"}); err != nil {
		t.Fatalf("the publication reached #worker@h and still failed: %v", err)
	}
	if !rep.has("a publication to news@h skipped bob@h, a User in a deliver-to group") {
		t.Fatalf("the skipped User was not said: %v", rep.lines)
	}
	b.mu.Lock()
	dropped, bobQueued := b.ensure("bob@h").dropped, len(b.ensure("bob@h").queue)
	b.mu.Unlock()
	if dropped != 0 || bobQueued != 0 {
		t.Fatalf("bob@h counted a drop (%d) or got a copy (%d)", dropped, bobQueued)
	}
}

// /group stores each member once and in canonical form, as /manage does (K.31).
func TestAGroupStoresEachMemberOnce(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h")
	if err := b.SetGroup("alice@h", "@inner@h", []string{"bob@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("alice@h", "@team@h", []string{"bob@h", " BOB@H ", "@Inner@H", "@inner@h", "bob@h"}); err != nil {
		t.Fatal(err)
	}
	if got := b.records["@team@h"].Allow; !slices.Equal(got, []string{"@inner@h", "bob@h"}) {
		t.Fatalf("the group stored %v", got)
	}
}

// A User's name has no template part: tmpl/eve is refused as a User, whether
// made as a profile or registered as a user record (K.32).
func TestAUserNameHasNoTemplatePart(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "admin@h")
	if _, err := b.SetUser("admin@h", protocol.User{Name: "tmpl/eve@h"}, true); !errors.Is(err, ErrProfile) {
		t.Fatalf("a template-form User was made: %v", err)
	}
	if _, known := b.users["tmpl/eve@h"]; known {
		t.Fatal("the refused User exists")
	}
	// Stored, one is ignored at start, with the user it names.
	restarted, rep := New(), &reports{}
	restarted.Journal(rep)
	assertRestoreRefused(t, restarted, canonicalFixture(ports.Snapshot{
		Users:   []protocol.User{{Name: "admin@h", Status: "active"}, {Name: "tmpl/eve@h", Status: "active"}},
		Records: []protocol.Record{userRecord("admin@h"), userRecord("tmpl/eve@h")},
	}, "admin@h"))
	// Falsifiable: a record of another kind may carry one.
	if _, err := b.Register(protocol.Record{Name: "#tmpl/eve@h", Owner: "admin@h", Kind: protocol.KindAgent}); err != nil {
		t.Fatalf("an agent instance was refused: %v", err)
	}
}
