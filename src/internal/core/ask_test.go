package core

import (
	"errors"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// An ask is help or mcp, with an empty body, on a message that is no
// receipt; anything else is refused (docs/constitution.md#agent-self-description).
func TestAnAskIsHelpOrMCPWithNoBody(t *testing.T) {
	b := rolesFixture(t)
	for _, bad := range []protocol.Envelope{
		{From: "bob@h", To: "#svc@h", Ask: "tools"},
		{From: "bob@h", To: "#svc@h", Ask: "Help"},
		{From: "bob@h", To: "#svc@h", Ask: protocol.AskHelp, Body: "--help"},
		{From: "bob@h", To: "#svc@h", Ask: protocol.AskMCP, Receipt: protocol.ReceiptDone, Re: "x"},
	} {
		if _, err := b.Send(bad); !errors.Is(err, ErrAsk) {
			t.Errorf("ask %q body %q receipt %q was taken: %v", bad.Ask, bad.Body, bad.Receipt, err)
		}
	}
	for _, ask := range []string{protocol.AskHelp, protocol.AskMCP} {
		if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "#svc@h", Ask: ask}); err != nil {
			t.Fatalf("ask %s: %v", ask, err)
		}
		if e := next(t, b, "#svc@h"); e.Ask != ask || e.Body != "" {
			t.Fatalf("ask %s arrived as ask %q body %q", ask, e.Ask, e.Body)
		}
	}
}

// An ask is kept through a forward, a published copy and a restart.
func TestAnAskTravelsThroughForwardsCopiesAndRestarts(t *testing.T) {
	b := rolesFixture(t)
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "jobs@h", Ask: protocol.AskHelp}); err != nil {
		t.Fatal(err)
	}
	if e := next(t, b, "#worker@h"); e.Ask != protocol.AskHelp || e.OriginalTo != "jobs@h" {
		t.Fatalf("the forward arrived with ask %q through %q", e.Ask, e.OriginalTo)
	}
	if _, err := b.Send(protocol.Envelope{From: "bob@h", To: "news@h", Ask: protocol.AskMCP}); err != nil {
		t.Fatal(err)
	}
	if e := next(t, b, "#worker@h"); e.Ask != protocol.AskMCP || e.OriginalTo != "news@h" {
		t.Fatalf("the published copy arrived with ask %q through %q", e.Ask, e.OriginalTo)
	}
	r := New()
	r.Restore(ports.Snapshot{
		Users:   []protocol.User{{Name: "alice@h", Status: "active"}},
		Records: []protocol.Record{userRecord("alice@h"), {Name: "#svc@h", Owner: "alice@h", Kind: protocol.KindAgent, Full: protocol.OverflowStrict}},
		Queues:  []ports.Queue{{Name: "#svc@h", Messages: []protocol.Envelope{{ID: "q", From: "alice@h", To: "#svc@h", Ask: protocol.AskHelp, At: time.Now()}}}},
	})
	if err := r.EstablishDaemonOwner("alice@h"); err != nil {
		t.Fatal(err)
	}
	if e := next(t, r, "#svc@h"); e.Ask != protocol.AskHelp {
		t.Fatalf("the restored message lost its ask: %q", e.Ask)
	}
}

// Manages answers by the rule Manage applies — the Owner, a Maintainer
// through a Group, the daemon Owner — and only about a record the caller
// may see (Plans/R1.2/slack/README.md#subscriptions-authority-errors).
func TestManagesAnswersByTheManageRule(t *testing.T) {
	b := rolesFixture(t)
	for who, want := range map[string]bool{"alice@h": true, "carol@h": true, "admin@h": true, "#svc@h": true, "bob@h": false, "nobody@h": false} {
		got, err := b.Manages("bob@h", who, "#svc@h")
		if err != nil || got != want {
			t.Errorf("does %s manage #svc@h: %v (%v), want %v", who, got, err, want)
		}
	}
	if _, err := b.Manages("eve@h", "alice@h", "#svc@h"); !errors.Is(err, ErrUnknown) {
		t.Errorf("a caller who may not see #svc@h was told: %v", err)
	}
}
