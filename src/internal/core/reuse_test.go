package core

import (
	"slices"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// A name freed by a record ignored at start inherits nothing: registered by
// somebody else, it is on no allow list, in no Group and maintains nothing
// that named the ignored record (K.23, docs/constitution.md#persistence-and-loading).
// A name nobody held keeps what names it in advance.
func TestANameFreedByAnIgnoredRecordInheritsNothing(t *testing.T) {
	st := memory.NewState()
	b := New()
	rep := &reports{}
	b.Journal(rep)
	b.Restore(ports.Snapshot{
		Users: []protocol.User{{Name: "alice@h", Status: "active"}, {Name: "eve@h", Status: "active"}},
		Records: []protocol.Record{
			userRecord("alice@h"), userRecord("eve@h"),
			// Owned by nobody, so ignored at load.
			{Name: "#lost@h", Owner: "ghost@h", Kind: protocol.KindAgent},
			{Name: "jobs@h", Owner: "alice@h", Kind: protocol.KindQueue, Allow: []string{"#lost@h", "#future@h"},
				Maintainers: protocol.MaintainerList{"#lost@h"}},
			{Name: "@crew@h", Owner: "alice@h", Kind: protocol.KindGroup, Allow: []string{"#lost@h", "eve@h"}},
		},
	})
	b.Persistence(st)
	if err := b.EstablishDaemonOwner("alice@h"); err != nil {
		t.Fatal(err)
	}
	if !rep.has("stored record #lost@h is ignored") {
		t.Fatalf("the fixture's record was not ignored: %v", rep.lines)
	}
	if _, err := b.Register(protocol.Record{Name: "#lost@h", Owner: "eve@h", Kind: protocol.KindAgent}); err != nil {
		t.Fatal(err)
	}
	jobs, crew := b.records["jobs@h"], b.records["@crew@h"]
	if slices.Contains(jobs.Allow, "#lost@h") || slices.Contains(jobs.Maintainers, "#lost@h") || slices.Contains(crew.Allow, "#lost@h") {
		t.Fatalf("the new #lost@h inherited: jobs allow %v, maintainers %v, @crew %v", jobs.Allow, jobs.Maintainers, crew.Allow)
	}
	if !slices.Contains(jobs.Allow, "#future@h") || !slices.Contains(crew.Allow, "eve@h") {
		t.Fatalf("references to other names went too: %v %v", jobs.Allow, crew.Allow)
	}
	if b.may("#lost@h", jobs) {
		t.Fatal("the new holder may use jobs@h")
	}
	// The cleanup is durable, in the same commit as the registration.
	saved, _ := st.Load()
	for _, r := range saved.Records {
		if (r.Name == "jobs@h" || r.Name == "@crew@h") && (slices.Contains(r.Allow, "#lost@h") || slices.Contains(r.Maintainers, "#lost@h")) {
			t.Fatalf("the store still holds %s naming #lost@h: %+v", r.Name, r)
		}
	}
	// Falsifiable: a name never held keeps its advance listing.
	if _, err := b.Register(protocol.Record{Name: "#future@h", Owner: "alice@h", Kind: protocol.KindAgent}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(b.records["jobs@h"].Allow, "#future@h") {
		t.Fatal("registering a name never held dropped its advance listing")
	}
}

// A User ignored at start for lacking its own record keeps its credential for
// the operator who repairs it: the start's ownerless sweep passes it by (K.26).
func TestAnIgnoredUserKeepsItsCredential(t *testing.T) {
	b := New()
	rep := &reports{}
	b.Journal(rep)
	b.Restore(ports.Snapshot{
		Users:   []protocol.User{{Name: "alice@h", Status: "active"}, {Name: "bob@h", Status: "active"}},
		Records: []protocol.Record{userRecord("alice@h")},
	})
	if err := b.EstablishDaemonOwner("alice@h"); err != nil {
		t.Fatal(err)
	}
	if !rep.has("stored user bob@h has no user record of its own and is ignored") {
		t.Fatalf("the fixture's user was not ignored: %v", rep.lines)
	}
	if swept := b.Ownerless([]string{"alice@h", "bob@h", "nobody@h"}); !slices.Equal(swept, []string{"nobody@h"}) {
		t.Fatalf("the sweep took %v; only a credential that answered for nothing goes", swept)
	}
}
