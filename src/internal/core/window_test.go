package core

import (
	"errors"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// A name whose record is removed while its own request is in flight arrives at
// the mutation as nobody, and the mutation it most wants is the one that would
// make it somebody again.
func TestARemovedCallerCannotRegisterItselfBack(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "#gone@h")
	if err := b.Unregister("#gone@h", "#gone@h"); err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{
		{Kind: protocol.KindAgent, Name: "#gone@h", Owner: "#gone@h"},
		{Kind: protocol.KindAgent, Name: "#gone@h"}, // owner defaulted to the name is the same claim
	} {
		if _, err := b.Register(r); !errors.Is(err, ErrNoPrincipal) {
			t.Fatalf("a name the daemon no longer knows registered itself back: %v", err)
		}
	}
	// And the legitimate caller for that shape still has it: enrolment has
	// proved a key, and says so rather than borrowing a clause anyone reaches.
	if _, err := b.register(protocol.Record{Kind: protocol.KindAgent, Name: "#gone@h", Owner: "#gone@h"}, true, false, ports.DirectoryProfile{}, nil); err != nil {
		t.Fatalf("enrolment could not write a newcomer its record: %v", err)
	}
}

// Ownership decided before the mint is ownership as it was, not as it is: a
// record changes hands, and the caller that owned it when it asked is handed
// the credential of the owner it now has.
func TestIssuingDoesNotHandOverACredentialOwnershipHasMovedOn(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "first@h", "second@h")
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "first@h"}); err != nil {
		t.Fatal(err)
	}
	mint := func(name string, _ ports.CredentialPair) (string, error) { return "credential for " + name, nil }
	if _, err := b.IssueFor("first@h", "#svc@h", mint); err != nil {
		t.Fatalf("the owner could not have its service's credential: %v", err)
	}
	owner := "second@h"
	if _, err := b.Manage("first@h", Management{Name: "#svc@h", Owner: &owner}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.IssueFor("first@h", "#svc@h", mint); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("the former owner was issued the current owner's credential: %v", err)
	}
	if _, err := b.IssueFor("second@h", "#svc@h", mint); err != nil {
		t.Fatalf("the new owner was refused its own service's credential: %v", err)
	}
	// The daemon owner and the name itself keep their standing routes.
	if _, err := b.IssueFor("admin@h", "#svc@h", mint); err != nil {
		t.Fatalf("the daemon owner was refused: %v", err)
	}
	if _, err := b.IssueFor("#svc@h", "#svc@h", mint); err != nil {
		t.Fatalf("a name was refused its own credential: %v", err)
	}
}

// And that decision is inside the hold the mint happens in, so a transfer
// cannot land between deciding and minting. Pinned by holding the mint open
// and showing the transfer cannot proceed meanwhile.
func TestIssuingHoldsTheRegistryWhileItDecidesAndMints(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "first@h", "second@h")
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "first@h"}); err != nil {
		t.Fatal(err)
	}
	minting, release := make(chan struct{}), make(chan struct{})
	issued := make(chan error, 1)
	go func() {
		_, err := b.IssueFor("first@h", "#svc@h", func(string, ports.CredentialPair) (string, error) {
			close(minting)
			<-release
			return "credential", nil
		})
		issued <- err
	}()
	<-minting

	owner := "second@h"
	transferring, transferred := make(chan struct{}), make(chan error, 1)
	go func() {
		close(transferring)
		_, err := b.Manage("first@h", Management{Name: "#svc@h", Owner: &owner})
		transferred <- err
	}()
	<-transferring
	select {
	case err := <-transferred:
		t.Fatalf("the record changed hands while its credential was being written: %v", err)
	case <-time.After(100 * time.Millisecond):
		// Correct: the transfer is waiting on the registry, as it must.
	}
	close(release)
	if err := <-issued; err != nil {
		t.Fatalf("issuing failed: %v", err)
	}
	if err := <-transferred; err != nil {
		t.Fatalf("the transfer did not proceed once issuing finished: %v", err)
	}
}

// A transfer hands a record to whoever is named, so that name has to be one
// who can answer for it now — not merely one who was registered once.
func TestATransferCannotHandARecordToSomebodyWhoCannotAct(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "owner@h")
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "owner@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetUser("admin@h", protocol.User{Name: "banned@h"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetUserState("admin@h", "banned@h", "inactive"); err != nil {
		t.Fatal(err)
	}
	to := "banned@h"
	if _, err := b.Manage("owner@h", Management{Name: "#svc@h", Owner: &to}); !errors.Is(err, ErrInactive) {
		t.Fatalf("a record was handed to somebody who cannot act: %v", err)
	}
	if r, _ := b.Lookup("owner@h", "#svc@h"); r.Owner != "owner@h" {
		t.Fatalf("the refused transfer moved the record anyway: owner is %s", r.Owner)
	}
}

// Removing the address and dropping its credential are one operation: the
// credential's removal is in the removal's own commit, and a commit that fails
// removes neither (docs/02-access.md#token-lifetime).
func TestRemovingAnAddressAndItsCredentialIsOneOperation(t *testing.T) {
	b := New()
	st := memory.NewState()
	b.Persistence(st)
	idx := newIndex()
	b.SetDaemonOwner("admin@h")
	known(t, b, "#svc@h")
	b.BindCredentials(idx)
	idx.held["#svc@h"] = mustPair(t, b, "#svc@h")
	if err := st.Tokens().Put(ports.Credential{Name: "#svc@h", Current: "x", CredentialPair: idx.held["#svc@h"]}); err != nil {
		t.Fatal(err)
	}
	st.Err = errors.New("the database is not writable")
	if err := b.Unregister("#svc@h", "#svc@h"); err == nil {
		t.Fatal("a failed commit was reported as a removal")
	}
	if _, ok := b.Lookup("#svc@h", "#svc@h"); !ok {
		t.Fatal("the record went even though the commit failed")
	}
	if len(idx.discarded) != 0 {
		t.Fatalf("the credential stopped working in a removal that failed: %v", idx.discarded)
	}
	if kept, _ := st.Tokens().Load(); len(kept) != 1 {
		t.Fatalf("a failed commit removed the stored credential: %+v", kept)
	}

	st.Err = nil
	if err := b.Unregister("#svc@h", "#svc@h"); err != nil {
		t.Fatal(err)
	}
	if len(idx.discarded) != 1 || idx.discarded[0] != "#svc@h" {
		t.Fatalf("the credential was not dropped with the address: %v", idx.discarded)
	}
	if kept, _ := st.Tokens().Load(); len(kept) != 0 {
		t.Fatalf("the removal's commit kept the credential row: %+v", kept)
	}

	// A person is the exception: their own record goes with them and they are
	// never removed, so the removal is refused and their credential kept.
	if _, err := b.SetUser("admin@h", protocol.User{Name: "someone@h"}, true); err != nil {
		t.Fatal(err)
	}
	idx.discarded = nil
	if err := b.Unregister("someone@h", "someone@h"); !errors.Is(err, ErrBusy) {
		t.Fatalf("a user's own record was removed: %v", err)
	}
	if len(idx.discarded) != 0 {
		t.Fatalf("a person was logged out: %v", idx.discarded)
	}
}

// The registry is still holding while the removal commits, so the name
// cannot be claimed by somebody else in between.
func TestRemovingHoldsTheRegistryWhileItCommits(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "#svc@h", "other@h")
	st := memory.NewState()
	st.Enter, st.Release = make(chan struct{}), make(chan struct{})
	b.Persistence(st)
	removed := make(chan error, 1)
	go func() { removed <- b.Unregister("#svc@h", "#svc@h") }()
	<-st.Enter

	claiming, claimed := make(chan struct{}), make(chan error, 1)
	go func() {
		close(claiming)
		_, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "other@h"})
		claimed <- err
	}()
	<-claiming
	select {
	case err := <-claimed:
		t.Fatalf("the name was claimed while its removal was committing: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(st.Release)
	if err := <-removed; err != nil {
		t.Fatalf("removal failed: %v", err)
	}
	if err := <-claimed; err != nil {
		t.Fatalf("the claim did not proceed once removal finished: %v", err)
	}
}
