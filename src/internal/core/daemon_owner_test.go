package core

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func ownerFixture(t *testing.T) *Bus {
	t.Helper()
	b := New()
	b.SetDaemonOwner("owner@h")
	for _, name := range []string{"alice@h", "next@h", "paused@h"} {
		if _, err := b.SetUser("owner@h", protocol.User{Name: name}, true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := b.SetUserState("owner@h", "paused@h", "paused"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "alice@h", Allow: []string{"alice@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "news@h", Owner: "alice@h", Kind: protocol.KindTopic, Mode: protocol.ModeQueue}); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDaemonOwnerManagesAndSeesEveryResourceWithoutOpeningItsACL(t *testing.T) {
	b := ownerFixture(t)
	if err := b.SetGroup("owner@h", AdministratorsGroup, []string{"owner@h", "admin@h"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"svc@h", "news@h"} {
		r, ok := b.Lookup("owner@h", name)
		if !ok || !r.CanManage || !r.CanTransfer {
			t.Fatalf("owner lacks node-wide controls for %s: %+v, %v", name, r, ok)
		}
		if _, ok := b.Lookup("admin@h", name); ok {
			t.Fatalf("Administrator inherited owner visibility for %s", name)
		}
		descr := "managed by node owner"
		if _, err := b.Manage("owner@h", Management{Name: name, Descr: &descr}); err != nil {
			t.Fatalf("owner manage %s: %v", name, err)
		}
		if _, err := b.Manage("admin@h", Management{Name: name, Descr: &descr}); !errors.Is(err, ErrNotOwner) {
			t.Fatalf("Administrator managed %s without record authority: %v", name, err)
		}
	}
	if _, err := b.Configure("svc@h", "owner@h", []byte(`{"root":true}`)); err != nil {
		t.Fatalf("owner could not configure another user's service: %v", err)
	}
	if _, err := b.Configure("svc@h", "admin@h", []byte(`{"root":false}`)); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("Administrator configured another user's service: %v", err)
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "news@h", Body: "closed"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("node management opened an empty ACL: %v", err)
	}
}

func TestDaemonOwnerTransferMovesRootAndImplicitMasterOnly(t *testing.T) {
	b := ownerFixture(t)
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "svc@h", Body: "before"}); err != nil {
		t.Fatalf("current owner's implicit master: %v", err)
	}
	if _, err := b.TransferDaemonOwner("owner@h", "owner@h"); !errors.Is(err, ErrBadName) {
		t.Fatalf("self transfer: %v", err)
	}
	if _, err := b.TransferDaemonOwner("owner@h", "missing@h"); !errors.Is(err, ErrBadName) {
		t.Fatalf("unknown recipient: %v", err)
	}
	if _, err := b.TransferDaemonOwner("owner@h", "paused@h"); !errors.Is(err, ErrInactive) {
		t.Fatalf("inactive recipient: %v", err)
	}
	if _, err := b.TransferDaemonOwner("next@h", "alice@h"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("non-owner transfer: %v", err)
	}
	if _, err := b.TransferDaemonOwner("owner@h", "next@h"); err != nil {
		t.Fatal(err)
	}
	if b.DaemonOwner() != "next@h" || !b.IsAdministrator("owner@h") || !b.IsAdministrator("next@h") {
		t.Fatalf("transfer nesting: owner=%s old-admin=%v new-admin=%v", b.DaemonOwner(), b.IsAdministrator("owner@h"), b.IsAdministrator("next@h"))
	}
	if members := b.Groups("next@h")[AdministratorsGroup]; !slices.Contains(members, "owner@h") || !slices.Contains(members, "next@h") {
		t.Fatalf("transfer did not preserve and add Administrator membership: %v", members)
	}
	if _, ok := b.Lookup("owner@h", "svc@h"); ok {
		t.Fatal("former owner retained node-wide visibility")
	}
	if r, ok := b.Lookup("next@h", "svc@h"); !ok || !r.CanManage || !r.CanTransfer {
		t.Fatalf("new owner lacks node-wide controls: %+v, %v", r, ok)
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "svc@h", Body: "old"}); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("former owner retained implicit master: %v", err)
	}
	if _, err := b.Send(protocol.Envelope{From: "next@h", To: "svc@h", Body: "after"}); err != nil {
		t.Fatalf("new owner lacks implicit master: %v", err)
	}
	if got := b.Recent("owner@h"); len(got) != 1 || got[0].From != "owner@h" {
		t.Fatalf("former owner retained node-wide recent view: %+v", got)
	}
	if got := b.Recent("next@h"); len(got) != 2 {
		t.Fatalf("new owner lacks node-wide recent view: %+v", got)
	}
}

func TestDaemonOwnerIsDurableAndLegacySnapshotsSeedOnce(t *testing.T) {
	legacy := New()
	legacy.Restore(ports.Snapshot{Clean: true})
	if err := legacy.EstablishDaemonOwner("seed@h"); err != nil {
		t.Fatal(err)
	}
	seeded := legacy.Snapshot()
	if !seeded.OwnerEstablished || seeded.Owner != "seed@h" {
		t.Fatalf("legacy seed was not made durable: %+v", seeded)
	}

	b := ownerFixture(t)
	if _, err := b.TransferDaemonOwner("owner@h", "next@h"); err != nil {
		t.Fatal(err)
	}
	restarted := New()
	restarted.Restore(b.Snapshot())
	if err := restarted.EstablishDaemonOwner("contrary-seed@h"); err != nil {
		t.Fatal(err)
	}
	if restarted.DaemonOwner() != "next@h" {
		t.Fatalf("startup seed replaced transferred owner: %s", restarted.DaemonOwner())
	}
}

func TestDamagedDurableOwnerFailsClosed(t *testing.T) {
	for name, snapshot := range map[string]ports.Snapshot{
		"missing":      {OwnerEstablished: true},
		"invalid":      {OwnerEstablished: true, Owner: "bad"},
		"unregistered": {OwnerEstablished: true, Owner: "gone@h"},
		"inactive": {
			OwnerEstablished: true,
			Owner:            "owner@h",
			Users:            []protocol.User{{Name: "owner@h", State: "paused"}},
		},
		"unmarked-owner": {Owner: "owner@h", Users: []protocol.User{{Name: "owner@h", State: "active"}}},
	} {
		t.Run(name, func(t *testing.T) {
			b := New()
			b.Restore(snapshot)
			err := b.EstablishDaemonOwner("seed@h")
			if err == nil || !strings.Contains(err.Error(), "owner") {
				t.Fatalf("damaged owner was silently reseeded: %v", err)
			}
		})
	}
}
