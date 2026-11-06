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
	if _, err := b.Configure(DaemonAgent, "owner@h", []byte(`{"realm":"early"}`)); !errors.Is(err, ErrUnknown) {
		t.Fatalf("pre-start configuration: %v", err)
	}
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
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
		if _, err := b.Configure(DaemonAgent, who, []byte(`{"stolen":true}`)); !errors.Is(err, ErrNotOwner) && !(who == "owner@h" && errors.Is(err, ErrConfig)) {
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
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	_, err := b.Send(protocol.Envelope{From: "alice@h", To: DaemonAgent, Body: "status"})
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
	assertRestoreRefused(t, restored, canonicalFixture(snap, ""))
}

// Q323: only #bus answers an asker whose Allow does not name it, and only an ask
// #bus took from its own inbox; any other answer needs the asker's Allow, and its
// refusal names the rule.
func TestOnlyBusAnswersAnyAskerAndOnlyAnAskItTook(t *testing.T) {
	b := ownerFixture(t)
	if err := b.EstablishDaemonAgent("node.example"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"#asker@h", "#helper@h"} {
		if _, err := b.Register(protocol.Record{Name: name, Kind: protocol.KindAgent, Owner: "owner@h"}); err != nil {
			t.Fatal(err)
		}
	}
	// An agent's answer is a send: the asker's Allow does not name #helper.
	_, err := b.Send(protocol.Envelope{From: "#helper@h", To: "#asker@h", Re: "0123456789abcdef", Body: "answer"})
	if !errors.Is(err, ErrNotAllow) || !strings.Contains(err.Error(), "only #bus answers any asker") {
		t.Fatalf("an agent's answer outside the asker's Allow: %v", err)
	}
	// An envelope #bus never took is no ask: a forged Re earns no exemption.
	for name, forged := range map[string]protocol.Envelope{
		"addressed elsewhere": {ID: "0123456789abcdef", From: "#asker@h", To: "#helper@h"},
		"no id":               {From: "#asker@h", To: DaemonAgent},
		"no asker":            {ID: "0123456789abcdef", To: DaemonAgent},
	} {
		if _, err := b.AnswerAsker(forged, "answer"); !errors.Is(err, ErrNotAllow) {
			t.Fatalf("%s: answered: %v", name, err)
		}
	}
	if r, _ := b.Lookup("owner@h", "#asker@h"); r.Queued != 0 {
		t.Fatalf("a refused answer was delivered: %d", r.Queued)
	}
	// The ask #bus took is answered, though the asker's Allow is empty.
	ask, err := b.Send(protocol.Envelope{From: "#asker@h", To: DaemonAgent, Body: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.AnswerAsker(ask, "ok"); err != nil {
		t.Fatalf("#bus could not answer its asker: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if got, err := b.Consume(ctx, "#asker@h", "", "", false, false); err != nil || got.Re != ask.ID || got.From != DaemonAgent {
		t.Fatalf("the asker did not get #bus's answer: %+v %v", got, err)
	}
}
