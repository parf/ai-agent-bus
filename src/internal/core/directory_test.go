package core

import (
	"errors"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestDirectoryClassifiesFactsAndPreservesCallerScope(t *testing.T) {
	b := New()
	b.Administrator("owner@h")
	b.Restore(ports.Snapshot{Users: []protocol.User{
		{Name: "smoke/person@h"}, // blank fields, test-like name, no record
		{Name: "paused@h", State: "paused"},
		{Name: "banned@h", State: "banned"},
	}})
	if err := b.SetGroup("owner@h", MaintainersGroup, []string{"owner@h", "maintainer@h"}, false); err != nil {
		t.Fatal(err)
	}
	known(t, b, "session@h")
	// Restored, not registered: a record owned by a name the daemon holds
	// nothing for but a credential can no longer be registered into existence
	// (docs/01-identity.md#when-the-owner-is-gone). It still arrives from an
	// older store, which is exactly why the directory has to show it.
	b.Restore(ports.Snapshot{Clean: true, Records: []protocol.Record{
		{Name: "owned@h", Owner: "unprofiled-owner@h", Kind: "generic", Full: protocol.OverflowStrict},
	}})
	credentials := []string{"session@h", "owned@h", "unprofiled-owner@h", "unused@h", "smoke/person@h"}
	for _, caller := range []string{"owner@h", "maintainer@h"} {
		users := b.Users(caller, credentials)
		got := map[string]protocol.User{}
		for _, u := range users {
			got[u.Name] = u
		}
		for name, kind := range map[string]string{
			"owner@h": protocol.DirectoryUser, "maintainer@h": protocol.DirectoryUser,
			"smoke/person@h": protocol.DirectoryUser, "paused@h": protocol.DirectoryUser,
			"banned@h": protocol.DirectoryUser, "session@h": protocol.DirectoryRecord,
			"unprofiled-owner@h": protocol.DirectoryCredential, "unused@h": protocol.DirectoryCredential,
		} {
			u, ok := got[name]
			if !ok || u.Kind != kind {
				t.Errorf("%s sees %s as %q, want %q", caller, name, u.Kind, kind)
			}
			if kind != protocol.DirectoryUser && (u.State != "" || u.CanEdit || u.CanActivate) {
				t.Errorf("non-user %s has invented lifecycle/controls: %+v", name, u)
			}
		}
		if _, ok := got["owned@h"]; ok {
			t.Error("ordinary owned services must stay in service listing")
		}
		if got["paused@h"].State != "paused" || got["banned@h"].State != "banned" {
			t.Error("non-active users were lost or relabelled")
		}
		if len(got["unprofiled-owner@h"].Services) != 1 {
			t.Error("retained credential lacks its owned-service explanation")
		}
	}
	for _, who := range []string{"smoke/person@h", "session@h"} {
		rows := b.Users(who, credentials)
		if len(rows) != 1 || rows[0].Name != who {
			t.Fatalf("ordinary caller %s can enumerate other identities: %+v", who, rows)
		}
	}
	// Not even its own row: a name the daemon holds nothing for but a
	// credential may not act, and looking itself up is acting
	// (docs/02-access.md#what-a-call-carries). It is in the directory for a
	// maintainer to see, which is the only reason it is there.
	if rows := b.Users("unused@h", credentials); len(rows) != 0 {
		t.Fatalf("a credential answering for nobody read the directory: %+v", rows)
	}
	if got := b.Ownerless(credentials); len(got) != 1 || got[0] != "unused@h" {
		t.Fatalf("classification disagrees with sweep: %v", got)
	}
	if rows := b.Users("owned@h", credentials); len(rows) != 0 {
		t.Fatalf("an ordinary owned service appears in the people directory: %+v", rows)
	}
	for _, state := range []string{"paused", "banned"} {
		if _, err := b.SetUserState("owner@h", "maintainer@h", state); err != nil {
			t.Fatal(err)
		}
		// Nothing at all now, not merely their own row without controls: a
		// suspended caller may not act, and reading the directory is acting.
		if rows := b.Users("maintainer@h", credentials); len(rows) != 0 {
			t.Fatalf("%s maintainer retains directory authority: %+v", state, rows)
		}
		called := false
		err := b.RemoveOwnerless("maintainer@h", "unused@h", func(string) error { called = true; return nil })
		// Refused for being suspended rather than for not being a maintainer,
		// because that is what is true and they are different answers to give
		// (docs/05-discovery.md#refusals).
		if !errors.Is(err, ErrInactive) || called {
			t.Errorf("%s maintainer can remove credentials: %v", state, err)
		}
	}
}

func TestCleanupSerializesRegistrationWithCredentialRemoval(t *testing.T) {
	for _, shape := range []string{"record", "profile"} {
		t.Run(shape, func(t *testing.T) {
			b := New()
			b.Administrator("owner@h")
			entered, finish := make(chan struct{}), make(chan struct{})
			removed := make(chan error, 1)
			go func() {
				removed <- b.RemoveOwnerless("owner@h", "unused@h", func(string) error {
					close(entered)
					<-finish // a credential-store write is still in progress
					return nil
				})
			}()
			<-entered
			attempted := make(chan struct{})
			registered := make(chan error, 1)
			go func() {
				close(attempted)
				if shape == "record" {
					_, err := b.Register(protocol.Record{Name: "unused@h", Owner: "owner@h"})
					registered <- err
				} else {
					_, err := b.SetUser("owner@h", protocol.User{Name: "unused@h"}, true)
					registered <- err
				}
			}()
			<-attempted
			early := false
			select {
			case <-registered:
				early = true
				t.Error("registration committed while an earlier cleanup could still delete its credential")
			case <-time.After(30 * time.Millisecond):
			}
			close(finish)
			if err := <-removed; err != nil {
				t.Fatal(err)
			}
			if !early {
				if err := <-registered; err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
