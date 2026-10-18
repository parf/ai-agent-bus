package core

import (
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestDirectoryClassifiesFactsAndPreservesCallerScope(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")
	b.Restore(ports.Snapshot{
		Users: []protocol.User{
			{Name: "smoke-person@h"}, // blank fields, test-like name
			{Name: "paused@h", Status: "inactive"},
			{Name: "banned@h", Status: "inactive"},
		},
		Records: []protocol.Record{userRecord("smoke-person@h"), userRecord("paused@h"), userRecord("banned@h")},
	})
	// An administrator is a User first (docs/01-identity-and-authority.md#daemon-administrators).
	if _, err := b.SetUser("owner@h", protocol.User{Name: "maintainer@h"}, true); err != nil {
		t.Fatal(err)
	}
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h", "maintainer@h"}); err != nil {
		t.Fatal(err)
	}
	known(t, b, "#session@h")
	credentials := []string{"#session@h", "unprofiled@h", "#unused@h", "smoke-person@h"}
	for _, caller := range []string{"owner@h", "maintainer@h"} {
		users := b.Users(caller, credentials)
		got := map[string]protocol.User{}
		for _, u := range users {
			got[u.Name] = u
		}
		for name, kind := range map[string]string{
			"owner@h": protocol.DirectoryUser, "maintainer@h": protocol.DirectoryUser,
			"smoke-person@h": protocol.DirectoryUser, "paused@h": protocol.DirectoryUser,
			"banned@h":     protocol.DirectoryUser,
			"unprofiled@h": protocol.DirectoryCredential, "#unused@h": protocol.DirectoryCredential,
		} {
			u, ok := got[name]
			if !ok || u.Kind != kind {
				t.Errorf("%s sees %s as %q, want %q", caller, name, u.Kind, kind)
			}
			if kind != protocol.DirectoryUser && (u.Status != "" || u.CanEdit || u.CanActivate) {
				t.Errorf("non-user %s has invented lifecycle/controls: %+v", name, u)
			}
		}
		if got["paused@h"].Status != "inactive" || got["banned@h"].Status != "inactive" {
			t.Error("non-active users were lost or relabelled")
		}
	}
	for caller := range map[string]bool{"owner@h": true, "maintainer@h": true} {
		for _, u := range b.Users(caller, credentials) {
			if u.Name == "#session@h" {
				t.Fatalf("an agent is a row in the people directory: %+v", u)
			}
		}
	}
	for _, who := range []string{"smoke-person@h"} {
		rows := b.Users(who, credentials)
		if len(rows) != 1 || rows[0].Name != who {
			t.Fatalf("ordinary caller %s can enumerate other identities: %+v", who, rows)
		}
	}
	if got := b.Ownerless(credentials); len(got) != 2 || got[0] != "#unused@h" || got[1] != "unprofiled@h" {
		t.Fatalf("classification disagrees with sweep: %v", got)
	}
}

func TestCleanupSerializesRegistrationWithCredentialRemoval(t *testing.T) {
	for _, shape := range []string{"record", "profile"} {
		t.Run(shape, func(t *testing.T) {
			// A record here is an agent's, spelled with its #; a profile is a
			// user's, spelled without.
			name := "#unused@h"
			if shape == "profile" {
				name = "unused@h"
			}
			b := New()
			b.SetDaemonOwner("owner@h")
			entered, finish := make(chan struct{}), make(chan struct{})
			removed := make(chan error, 1)
			go func() {
				removed <- b.RemoveOwnerless("owner@h", name, func(string) error {
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
					_, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: name, Owner: "owner@h"})
					registered <- err
				} else {
					_, err := b.SetUser("owner@h", protocol.User{Name: name}, true)
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
