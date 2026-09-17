package core

import (
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func accountFixture(t *testing.T) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("owner@h")
	for _, name := range []string{"alice@h", "bob@h", "ordinary@h"} {
		if _, err := b.SetUser("owner@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h", "alice@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.EstablishAccounts(map[string]string{"alice-os": "alice@h", "other-os": "ordinary@h"}); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAccountMapIsAdministrativeDurableAndAppliedOnlyByRestart(t *testing.T) {
	b := accountFixture(t)
	view, err := b.Accounts("alice@h")
	if err != nil || view.RestartRequired || len(view.Mappings) != 2 {
		t.Fatalf("initial active map: %+v, %v", view, err)
	}
	if _, err := b.SetAccount("ordinary@h", "alice-os", "bob@h", false); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("ordinary user changed account map: %v", err)
	}
	view, err = b.SetAccount("alice@h", "alice-os", "bob@h", false)
	if err != nil || !view.RestartRequired {
		t.Fatalf("saved change did not require restart: %+v, %v", view, err)
	}
	if got := b.activeAccounts["alice-os"]; got != "alice@h" {
		t.Fatalf("an in-process edit rewrote the active listener: %q", got)
	}

	saved := b.Snapshot()
	clone := New()
	clone.Restore(saved)
	// The old setup flag is contrary on purpose. A current snapshot wins.
	if err := clone.EstablishAccounts(map[string]string{"alice-os": "alice@h", "other-os": "ordinary@h"}); err != nil {
		t.Fatal(err)
	}
	view, err = clone.Accounts("owner@h")
	if err != nil || !view.RestartRequired || len(view.Mappings) != 2 || view.Mappings[0].Principal != "bob@h" {
		t.Fatalf("snapshot did not remain authoritative: %+v, %v", view, err)
	}
	// What the supervisor opened from that snapshot is the desired map on the
	// next full start, at which point the restart warning clears.
	active := map[string]string{"alice-os": "bob@h", "other-os": "ordinary@h"}
	if err := clone.EstablishAccounts(active); err != nil {
		t.Fatal(err)
	}
	view, err = clone.Accounts("owner@h")
	if err != nil || view.RestartRequired {
		t.Fatalf("applied map still claims a restart: %+v, %v", view, err)
	}
}

func TestAccountMapRefusesUnknownInactiveAndDamagedState(t *testing.T) {
	b := accountFixture(t)
	if _, err := b.SetAccount("owner@h", "new-os", "unknown@h", false); !errors.Is(err, ErrNoPrincipal) {
		t.Fatalf("unknown principal was mapped: %v", err)
	}
	if _, err := b.SetUserState("owner@h", "bob@h", "paused"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetAccount("owner@h", "new-os", "bob@h", false); !errors.Is(err, ErrInactive) {
		t.Fatalf("inactive principal was mapped: %v", err)
	}
	if _, err := b.SetAccount("owner@h", "missing-os", "", true); !errors.Is(err, ErrUnknown) {
		t.Fatalf("missing mapping removal: %v", err)
	}

	for _, damaged := range []ports.Snapshot{
		{Accounts: []protocol.AccountMapping{{Account: "os", Principal: "alice@h"}}},
		{AccountsEstablished: true, Accounts: []protocol.AccountMapping{{Account: "", Principal: "alice@h"}}},
		{AccountsEstablished: true, Accounts: []protocol.AccountMapping{{Account: "os", Principal: "bad"}}},
		{AccountsEstablished: true, Accounts: []protocol.AccountMapping{{Account: "os", Principal: "alice@h"}, {Account: "os", Principal: "bob@h"}}},
	} {
		clone := New()
		clone.Restore(damaged)
		if err := clone.EstablishAccounts(nil); err == nil {
			t.Fatalf("damaged account map started: %+v", damaged)
		}
	}
}

func TestAccountMapRemovalPersistsAsAnIntentionallyEmptyMap(t *testing.T) {
	b := New()
	b.SetDaemonOwner("owner@h")
	if err := b.EstablishAccounts(map[string]string{"one": "owner@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.SetAccount("owner@h", "one", "", true); err != nil {
		t.Fatal(err)
	}
	saved := b.Snapshot()
	if !saved.AccountsEstablished || len(saved.Accounts) != 0 {
		t.Fatalf("empty map was not explicit: %+v", saved.Accounts)
	}
	clone := New()
	clone.Restore(saved)
	if err := clone.EstablishAccounts(map[string]string{"one": "owner@h"}); err != nil {
		t.Fatal(err)
	}
	if err := clone.EstablishDaemonOwner("owner@h"); err != nil {
		t.Fatal(err)
	}
	view, err := clone.Accounts("owner@h")
	if err != nil || len(view.Mappings) != 0 || !view.RestartRequired {
		t.Fatalf("legacy seed resurrected removed mapping: %+v, %v", view, err)
	}
}
