package core

import (
	"errors"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"testing"
)

func TestUserCreationCannotCreateGroupOrRuntimePrincipals(t *testing.T) {
	b := accountFixture(t)
	for _, caller := range []string{"owner@h", "alice@h"} {
		for _, name := range []string{"@owner", "@administrators", "@team"} {
			if _, err := b.SetUser(caller, protocol.User{Name: name}, true); !errors.Is(err, ErrProfile) {
				t.Fatalf("%s created %s: %v", caller, name, err)
			}
			if _, ok := b.records[name]; ok && name != "@administrators" {
				t.Fatalf("invalid user %s reached storage", name)
			}
			if _, ok := b.users[name]; ok {
				t.Fatalf("invalid user %s reached the profile table", name)
			}
		}
	}
	if _, err := b.SetUser("alice@h", protocol.User{Name: "newperson@h"}, true); err != nil {
		t.Fatal(err)
	}
	clone := New()
	clone.Restore(b.Snapshot())
	if err := clone.EstablishDaemonOwner("owner@h"); err != nil {
		t.Fatalf("refused names left an unrestorable snapshot: %v", err)
	}
}
