package core

import (
	"errors"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// Authority is decided against the record as it is now, never against the
// change being made (docs/constitution.md#authority-rules).

func authorityFixture(t *testing.T) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h", "mallory@h", "maint@h")
	provision(t, b, nil,
		protocol.Record{Name: "#svc@h", Kind: protocol.KindAgent, Owner: "alice@h", Maintainers: protocol.MaintainerList{"maint@h"}},
		protocol.Record{Name: "jobs@h", Kind: protocol.KindQueue, Owner: "alice@h", Maintainers: protocol.MaintainerList{"maint@h"}},
	)
	return b
}

// The old Owner transfers; the new one governs at once, and the old one not
// at all.
func TestANewOwnerGovernsAtOnce(t *testing.T) {
	b := authorityFixture(t)
	bob, descr := "bob@h", "bob's now"
	if _, err := b.Manage("bob@h", Management{Name: "jobs@h", Owner: &bob}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("the recipient took the record for himself: %v", err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Owner: &bob}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("bob@h", Management{Name: "jobs@h", Descr: &descr}); err != nil {
		t.Fatalf("the new owner cannot govern: %v", err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Descr: &descr}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("the old owner still governs: %v", err)
	}
	alice := "alice@h"
	if _, err := b.Manage("alice@h", Management{Name: "jobs@h", Owner: &alice}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("the old owner took it back: %v", err)
	}
}

// A submitted grant cannot authorize the write that submits it.
func TestAGrantCannotAuthorizeItself(t *testing.T) {
	b := authorityFixture(t)
	self := protocol.MaintainerList{"mallory@h"}
	if _, err := b.Manage("mallory@h", Management{Name: "jobs@h", Maintainers: &self}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a stranger named herself Maintainer: %v", err)
	}
	allow := []string{"mallory@h"}
	if _, err := b.Manage("mallory@h", Management{Name: "jobs@h", Allow: &allow}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a stranger admitted herself: %v", err)
	}
	mallory := "mallory@h"
	if _, err := b.Manage("mallory@h", Management{Name: "jobs@h", Owner: &mallory}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a stranger transferred the record to herself: %v", err)
	}
	if r, _ := b.Lookup("alice@h", "jobs@h"); r.Owner != "alice@h" || len(r.Allow) != 0 || len(r.Maintainers) != 1 {
		t.Fatalf("a refused self-grant changed the record: %+v", r)
	}
}

// A Maintainer edits the operational fields and none of the protected ones.
func TestAMaintainerCannotAlterProtectedFields(t *testing.T) {
	b := authorityFixture(t)
	descr, allow := "maintained", []string{"bob@h"}
	if _, err := b.Manage("maint@h", Management{Name: "jobs@h", Descr: &descr, Allow: &allow}); err != nil {
		t.Fatalf("a Maintainer cannot edit the description and ACL: %v", err)
	}
	bob, personal := "bob@h", true
	maintainers := protocol.MaintainerList{"maint@h", "bob@h"}
	for what, change := range map[string]Management{
		"owner":       {Name: "jobs@h", Owner: &bob},
		"maintainers": {Name: "jobs@h", Maintainers: &maintainers},
		"personal":    {Name: "jobs@h", Personal: &personal},
	} {
		if _, err := b.Manage("maint@h", change); !errors.Is(err, ErrNotOwner) {
			t.Errorf("a Maintainer changed the %s: %v", what, err)
		}
	}
	if r, _ := b.Lookup("alice@h", "jobs@h"); r.Owner != "alice@h" || r.Personal || len(r.Maintainers) != 1 {
		t.Fatalf("a Maintainer's refused edit changed a protected field: %+v", r)
	}
}

// An Agent manages its own record as a Maintainer would, and no more; Personal
// is its Owner's alone.
func TestAnAgentManagesItsOwnRecordAsAMaintainer(t *testing.T) {
	b := authorityFixture(t)
	descr := "self-described"
	if _, err := b.Manage("#svc@h", Management{Name: "#svc@h", Descr: &descr}); err != nil {
		t.Fatalf("an agent cannot describe itself: %v", err)
	}
	bob, personal := "bob@h", true
	maintainers := protocol.MaintainerList{"#svc@h"}
	for what, change := range map[string]Management{
		"owner":       {Name: "#svc@h", Owner: &bob},
		"maintainers": {Name: "#svc@h", Maintainers: &maintainers},
		"personal":    {Name: "#svc@h", Personal: &personal},
	} {
		if _, err := b.Manage("#svc@h", change); !errors.Is(err, ErrNotOwner) {
			t.Errorf("an agent changed its own %s: %v", what, err)
		}
	}
	// The Owner can, in one write that also drops the Maintainer a Personal
	// record could not keep: the final record is what is checked.
	if _, err := b.Manage("alice@h", Management{Name: "#svc@h", Personal: &personal}); !errors.Is(err, ErrPersonal) {
		t.Fatalf("Personal was set with a Maintainer outside the cohort: %v", err)
	}
	if _, err := b.Manage("alice@h", Management{Name: "#svc@h", Personal: &personal, Maintainers: &protocol.MaintainerList{}}); err != nil {
		t.Fatalf("the Owner cannot make her agent Personal: %v", err)
	}
}

// A revocation and a write by the revoked are ordered: once the revocation is
// committing, the Maintainer's edit waits for it and is then judged against
// the state it left.
func TestARevocationCannotRaceAStaleAuthorizationIntoAWrite(t *testing.T) {
	b := authorityFixture(t)
	st := memory.NewState()
	st.Enter, st.Release = make(chan struct{}), make(chan struct{})
	b.Persistence(st)
	revoked := make(chan error, 1)
	none := protocol.MaintainerList{}
	go func() {
		_, err := b.Manage("alice@h", Management{Name: "jobs@h", Maintainers: &none})
		revoked <- err
	}()
	<-st.Enter
	edited := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		descr := "stale"
		_, err := b.Manage("maint@h", Management{Name: "jobs@h", Descr: &descr})
		edited <- err
	}()
	<-started
	select {
	case err := <-edited:
		t.Fatalf("the Maintainer's edit ran while its revocation was committing: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(st.Release)
	if err := <-revoked; err != nil {
		t.Fatal(err)
	}
	if err := <-edited; !errors.Is(err, ErrNotOwner) {
		t.Fatalf("the revoked Maintainer's edit landed: %v", err)
	}
	if r, _ := b.Lookup("alice@h", "jobs@h"); r.Descr == "stale" {
		t.Fatal("the stale authorization wrote")
	}
}
