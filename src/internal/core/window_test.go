package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Every verb asks who the caller is where it acts, not only at the edge.
//
// The gate releases its lock before core takes one, so a caller can stop being
// a principal, or be suspended, in between. This is the whole class in one
// table: each verb is called by a name the daemon holds nothing for, and by one
// that has been paused, and each has to refuse with which of the two it is —
// the codes are different and a caller acts on them
// (docs/05-discovery.md#refusals).
func TestEveryVerbAsksWhoTheCallerIsWhereItActs(t *testing.T) {
	verbs := []struct {
		name string
		call func(b *Bus, caller string) error
	}{
		{"register", func(b *Bus, c string) error {
			_, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "fresh@h", Owner: c})
			return err
		}},
		{"register-self", func(b *Bus, c string) error {
			_, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: c, Owner: c})
			return err
		}},
		{"configure", func(b *Bus, c string) error {
			_, err := b.Configure("target@h", c, json.RawMessage(`{"k":1}`))
			return err
		}},
		{"config", func(b *Bus, c string) error { _, err := b.Config(c, c); return err }},
		{"unregister", func(b *Bus, c string) error { return b.Unregister("target@h", c) }},
		{"send", func(b *Bus, c string) error {
			_, err := b.Send(protocol.Envelope{From: c, To: "target@h", Body: "x"})
			return err
		}},
		{"subscribe", func(b *Bus, c string) error { _, err := b.Subscribe(c, "news@h", true); return err }},
		{"consume", func(b *Bus, c string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			_, err := b.ConsumeAs(ctx, c, "target@h", "", "", false, false)
			return err
		}},
		{"manage", func(b *Bus, c string) error {
			descr := "mine now"
			_, err := b.Manage(c, Management{Name: "target@h", Descr: &descr})
			return err
		}},
		{"remove-subscriber", func(b *Bus, c string) error {
			_, err := b.RemoveSubscriber(c, "news@h", "sub@h")
			return err
		}},
		{"set-group", func(b *Bus, c string) error { return b.SetGroup(c, "@team", []string{c}) }},
		{"set-user", func(b *Bus, c string) error {
			_, err := b.SetUser(c, protocol.User{Name: "someone@h"}, true)
			return err
		}},
		{"set-user-state", func(b *Bus, c string) error {
			_, err := b.SetUserState(c, "someone@h", "paused")
			return err
		}},
		{"remove-ownerless", func(b *Bus, c string) error {
			return b.RemoveOwnerless(c, "junk@h", func(string) error {
				t.Error("a credential was dropped for a caller that may not act")
				return nil
			})
		}},
		{"issue", func(b *Bus, c string) error {
			_, err := b.IssueFor(c, "target@h", func(string) (string, error) {
				t.Error("a credential was minted for a caller that may not act")
				return "tok", nil
			})
			return err
		}},
	}
	for _, v := range verbs {
		for _, caller := range []struct {
			who  string
			want error
		}{
			{"nobody@h", ErrNoPrincipal},
			{"paused@h", ErrInactive},
		} {
			t.Run(v.name+"/"+caller.who, func(t *testing.T) {
				b := New()
				b.SetDaemonOwner("admin@h")
				known(t, b, "target@h", "sub@h")
				provision(t, b, protocol.Record{
					Name: "news@h", Kind: protocol.KindPubSub,
				})
				// Paused is a maintainer, so nothing it is refused for can be
				// mistaken for lacking authority: it had all of it a moment ago.
				if _, err := b.SetUser("admin@h", protocol.User{Name: "paused@h"}, true); err != nil {
					t.Fatal(err)
				}
				if err := b.SetGroup("admin@h", AdministratorsGroup, []string{"admin@h", "paused@h"}); err != nil {
					t.Fatal(err)
				}
				if _, err := b.SetUserState("admin@h", "paused@h", "paused"); err != nil {
					t.Fatal(err)
				}
				if err := v.call(b, caller.who); !errors.Is(err, caller.want) {
					t.Fatalf("%s by %s: err = %v, want %v", v.name, caller.who, err, caller.want)
				}
			})
		}
	}
}

// A name whose record is removed while its own request is in flight arrives at
// the mutation as nobody, and the mutation it most wants is the one that would
// make it somebody again.
func TestARemovedCallerCannotRegisterItselfBack(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "gone@h")
	if err := b.Unregister("gone@h", "gone@h"); err != nil {
		t.Fatal(err)
	}
	for _, r := range []protocol.Record{
		{Kind: protocol.KindAgent, Name: "gone@h", Owner: "gone@h"},
		{Kind: protocol.KindAgent, Name: "gone@h"}, // owner defaulted to the name is the same claim
	} {
		if _, err := b.Register(r); !errors.Is(err, ErrNoPrincipal) {
			t.Fatalf("a name the daemon no longer knows registered itself back: %v", err)
		}
	}
	// And the legitimate caller for that shape still has it: enrolment has
	// proved a key, and says so rather than borrowing a clause anyone reaches.
	if _, err := b.register(protocol.Record{Kind: protocol.KindAgent, Name: "gone@h", Owner: "gone@h"}, true, false, ports.DirectoryProfile{}); err != nil {
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
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "svc@h", Owner: "first@h"}); err != nil {
		t.Fatal(err)
	}
	mint := func(name string) (string, error) { return "credential for " + name, nil }
	if _, err := b.IssueFor("first@h", "svc@h", mint); err != nil {
		t.Fatalf("the owner could not have its service's credential: %v", err)
	}
	owner := "second@h"
	if _, err := b.Manage("first@h", Management{Name: "svc@h", Owner: &owner}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.IssueFor("first@h", "svc@h", mint); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("the former owner was issued the current owner's credential: %v", err)
	}
	if _, err := b.IssueFor("second@h", "svc@h", mint); err != nil {
		t.Fatalf("the new owner was refused its own service's credential: %v", err)
	}
	// The daemon owner and the name itself keep their standing routes.
	if _, err := b.IssueFor("admin@h", "svc@h", mint); err != nil {
		t.Fatalf("the daemon owner was refused: %v", err)
	}
	if _, err := b.IssueFor("svc@h", "svc@h", mint); err != nil {
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
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "svc@h", Owner: "first@h"}); err != nil {
		t.Fatal(err)
	}
	minting, release := make(chan struct{}), make(chan struct{})
	issued := make(chan error, 1)
	go func() {
		_, err := b.IssueFor("first@h", "svc@h", func(string) (string, error) {
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
		_, err := b.Manage("first@h", Management{Name: "svc@h", Owner: &owner})
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
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "svc@h", Owner: "owner@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetUser("admin@h", protocol.User{Name: "banned@h"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetUserState("admin@h", "banned@h", "banned"); err != nil {
		t.Fatal(err)
	}
	to := "banned@h"
	if _, err := b.Manage("owner@h", Management{Name: "svc@h", Owner: &to}); !errors.Is(err, ErrInactive) {
		t.Fatalf("a record was handed to somebody who cannot act: %v", err)
	}
	if r, _ := b.Lookup("owner@h", "svc@h"); r.Owner != "owner@h" {
		t.Fatalf("the refused transfer moved the record anyway: owner is %s", r.Owner)
	}
}

// Removing the address and dropping its credential are one operation, and the
// order within it is the one that cannot strand anything: a store that will
// not take the removal abandons the whole thing, rather than leaving a record
// gone and a credential answering for it.
func TestRemovingAnAddressAndItsCredentialIsOneOperation(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "svc@h")
	refuse := errors.New("the credential store is not writable")
	if err := b.UnregisterAnd("svc@h", "svc@h", func(string) error { return refuse }); !errors.Is(err, refuse) {
		t.Fatalf("a failed credential write was not reported: %v", err)
	}
	if _, ok := b.Lookup("svc@h", "svc@h"); !ok {
		t.Fatal("the record went even though its credential could not be dropped")
	}
	if b.Authenticate("svc@h") != nil {
		t.Fatal("the name stopped being a principal in a removal that failed")
	}

	dropped := []string{}
	if err := b.UnregisterAnd("svc@h", "svc@h", func(n string) error {
		dropped = append(dropped, n)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(dropped) != 1 || dropped[0] != "svc@h" {
		t.Fatalf("the credential was not dropped with the address: %v", dropped)
	}

	// A person is the exception, decided on the same facts the removal is:
	// their credential is how they call at all.
	if _, err := b.SetUser("admin@h", protocol.User{Name: "someone@h"}, true); err != nil {
		t.Fatal(err)
	}
	dropped = dropped[:0]
	if err := b.UnregisterAnd("someone@h", "someone@h", func(n string) error {
		dropped = append(dropped, n)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(dropped) != 0 {
		t.Fatalf("a person was logged out by giving up a record: %v", dropped)
	}
}

// The registry is still holding while the credential store is written, so the
// name cannot be claimed by somebody else in between and have *their*
// credential dropped instead.
func TestRemovingHoldsTheRegistryWhileItDropsTheCredential(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "svc@h", "other@h")
	forgetting, release := make(chan struct{}), make(chan struct{})
	removed := make(chan error, 1)
	go func() {
		removed <- b.UnregisterAnd("svc@h", "svc@h", func(string) error {
			close(forgetting)
			<-release
			return nil
		})
	}()
	<-forgetting

	claiming, claimed := make(chan struct{}), make(chan error, 1)
	go func() {
		close(claiming)
		_, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "svc@h", Owner: "other@h"})
		claimed <- err
	}()
	<-claiming
	select {
	case err := <-claimed:
		t.Fatalf("the name was claimed while its credential was being dropped: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-removed; err != nil {
		t.Fatalf("removal failed: %v", err)
	}
	if err := <-claimed; err != nil {
		t.Fatalf("the claim did not proceed once removal finished: %v", err)
	}
}

// A blocked read outlives the request that started it, so a principal that is
// removed while reading is being served on standing nobody has any more.
func TestARemovedPrincipalsBlockedReadIsReleased(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "reader@h")
	provision(t, b, protocol.Record{Kind: protocol.KindAgent, Name: "shared@h", Owner: "admin@h", Allow: []string{"reader@h"}})
	stopped := make(chan error, 1)
	go func() {
		_, err := b.ConsumeAs(context.Background(), "reader@h", "shared@h", "", "", false, false)
		stopped <- err
	}()
	waitForWaiters(t, b, "shared@h", 1)
	if err := b.Unregister("reader@h", "reader@h"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-stopped:
		if !errors.Is(err, ErrNoPrincipal) {
			t.Fatalf("the read ended for the wrong reason: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a removed principal is still being read for")
	}
}

// The listings answer with what the caller may see, and a caller that may not
// act may see nothing. They have no error to return, so the refusal is the
// empty answer — which is the same thing discovery does with a name you are not
// allowed to know about (docs/02-access.md#acl).
func TestListingsShowNothingToACallerThatMayNotAct(t *testing.T) {
	for _, caller := range []string{"nobody@h", "paused@h"} {
		t.Run(caller, func(t *testing.T) {
			b := New()
			b.SetDaemonOwner("admin@h")
			known(t, b, "svc@h")
			if _, err := b.SetUser("admin@h", protocol.User{Name: "paused@h"}, true); err != nil {
				t.Fatal(err)
			}
			// A maintainer, so that what it is refused cannot be mistaken for
			// never having had the authority.
			if err := b.SetGroup("admin@h", AdministratorsGroup, []string{"admin@h", "paused@h"}); err != nil {
				t.Fatal(err)
			}
			if _, err := b.SetUserState("admin@h", "paused@h", "paused"); err != nil {
				t.Fatal(err)
			}
			if _, err := b.Send(protocol.Envelope{From: "svc@h", To: "svc@h", Body: "x"}); err != nil {
				t.Fatal(err)
			}
			// Administrative standing does not bypass inactivity. The history
			// question is asked of a caller whose role would otherwise expose
			// the node feed, which is the standing this is about losing.
			if got := b.List(caller, ""); len(got) != 0 {
				t.Errorf("list: %+v", got)
			}
			if _, ok := b.Lookup(caller, "svc@h"); ok {
				t.Error("lookup answered")
			}
			if got := b.Owned(caller); len(got) != 0 {
				t.Errorf("owned: %v", got)
			}
			if got := b.Recent(caller); len(got) != 0 {
				t.Errorf("recent: %+v", got)
			}
			if got := b.Groups(caller); len(got) != 0 {
				t.Errorf("groups: %v", got)
			}
			if got := b.Users(caller, []string{"svc@h"}); len(got) != 0 {
				t.Errorf("users: %+v", got)
			}
			if points, err := b.Activity(caller, "svc@h"); err == nil && len(points) != 0 {
				t.Errorf("activity: %+v %v", points, err)
			}
		})
	}
}
