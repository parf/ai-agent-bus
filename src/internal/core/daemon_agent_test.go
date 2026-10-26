package core

import (
	"context"
	"errors"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"strings"
	"testing"
	"time"
)

func TestDaemonAgentIsReservedAndUncredentialed(t *testing.T) {
	b := ownerFixture(t)
	if _, err := b.Register(protocol.Record{Name: DaemonAgent, Kind: protocol.KindAgent, Owner: "owner@h"}); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("pre-start registration: %v", err)
	}
	if _, err := b.Configure(DaemonAgent, "owner@h", []byte(`{"x":1}`)); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("pre-start configuration: %v", err)
	}
	if err := b.EstablishDaemonAgent(); err != nil {
		t.Fatal(err)
	}
	r, ok := b.Lookup("alice@h", DaemonAgent)
	if !ok || r.Name != "#bus" || r.CanManage || r.CanTransfer {
		t.Fatalf("not a protected visible identity: %+v", r)
	}
	for _, who := range []string{"owner@h", "alice@h"} {
		if _, err := b.Register(protocol.Record{Name: DaemonAgent, Kind: protocol.KindAgent, Owner: who}); !errors.Is(err, ErrNotOwner) {
			t.Fatalf("register %s: %v", who, err)
		}
		descr := "stolen"
		if _, err := b.Manage(who, Management{Name: DaemonAgent, Descr: &descr}); !errors.Is(err, ErrNotOwner) {
			t.Fatalf("manage %s: %v", who, err)
		}
		if err := b.Unregister(DaemonAgent, who); !errors.Is(err, ErrNotOwner) {
			t.Fatalf("unregister %s: %v", who, err)
		}
		if _, err := b.Configure(DaemonAgent, who, []byte(`{"stolen":true}`)); !errors.Is(err, ErrNotOwner) {
			t.Fatalf("configure %s: %v", who, err)
		}
		if _, err := b.SetSecret(DaemonAgent, who, "TOKEN=stolen"); !errors.Is(err, ErrNotOwner) {
			t.Fatalf("Secret %s: %v", who, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		if _, err := b.ConsumeAddressed(ctx, who, DaemonAgent, "", "", "", false, false); !errors.Is(err, ErrNotAllow) {
			t.Fatalf("consume %s: %v", who, err)
		}
		cancel()
		if known, may := b.LockAccess(who, DaemonAgent); !known || may {
			t.Fatalf("lock access: %s %v %v", who, known, may)
		}
	}
	if err := b.Authenticate(DaemonAgent); !errors.Is(err, ErrNoPrincipal) {
		t.Fatalf("socket identity: %v", err)
	}
	if _, err := b.PairFor(DaemonAgent); !errors.Is(err, ErrKind) {
		t.Fatalf("credential: %v", err)
	}
	if _, err := b.SetAccount("owner@h", "stolen", DaemonAgent, false); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("account alias: %v", err)
	}
	if _, err := b.TransferDaemonOwner("owner@h", "next@h"); err != nil {
		t.Fatal(err)
	}
	r, ok = b.Lookup("next@h", DaemonAgent)
	if !ok || r.Owner != "next@h" || r.CanManage {
		t.Fatalf("owner transfer: %+v", r)
	}
}

func TestDaemonAgentRestoreKeepsQueueButRejectsStoredTakeover(t *testing.T) {
	b := ownerFixture(t)
	if err := b.EstablishDaemonAgent(); err != nil {
		t.Fatal(err)
	}
	e, err := b.Send(protocol.Envelope{From: "alice@h", To: DaemonAgent, Body: "status"})
	if err != nil {
		t.Fatal(err)
	}
	snap := b.Snapshot()
	for i, r := range snap.Records {
		if r.Name == DaemonAgent {
			r.Owner = "alice@h"
			r.Allow = []string{"alice@h"}
			r.Subs = []string{"news@h"}
			r.Secret = "TOKEN=stolen"
			r.Status = protocol.StatusInactive
			snap.Records[i] = r
		}
	}
	journal := &reports{}
	restored := New()
	restored.Journal(journal)
	restored.Restore(snap)
	if !strings.Contains(strings.Join(journal.lines, "\n"), "stored #bus metadata conflicts") {
		t.Fatal("stored takeover was not reported")
	}
	if err := restored.EstablishDaemonOwner("owner@h"); err != nil {
		t.Fatal(err)
	}
	if r, ok := restored.Lookup("alice@h", DaemonAgent); !ok || r.Owner != "owner@h" || len(r.Subs) != 0 {
		t.Fatalf("restore exposed takeover before startup: %+v", r)
	}
	if err := restored.EstablishDaemonAgent(); err != nil {
		t.Fatal(err)
	}
	r, ok := restored.Lookup("alice@h", DaemonAgent)
	if !ok || r.Owner != "owner@h" || strings.Join(r.Allow, ",") != "*" || len(r.Subs) != 0 || r.Secret != "" || r.SecretSHA != "" {
		t.Fatalf("stored takeover survived: %+v", r)
	}
	got, err := restored.Consume(context.Background(), DaemonAgent, "", "", false, false)
	if err != nil || got.ID != e.ID {
		t.Fatalf("durable request lost: %+v %v", got, err)
	}
}
