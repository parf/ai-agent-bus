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

// A removed name keeps nothing: no reservation, no owner, no claim on the
// name. Reserving it cost a permanent entry per throwaway address and bought
// protection MVP does not need; it is R1.2's
// (../../Plans/R1.2/README.md#removed-names).
func TestUnregisterLeavesNothingBehind(t *testing.T) {
	b := New()
	person(b, "owner@h", "stranger@h")
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "owner@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("#svc@h", "stranger@h"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("stranger removed record: %v", err)
	}
	before := b.Status().Services
	if err := b.Unregister(" #SVC@H ", " OWNER@H "); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.Lookup("owner@h", "#svc@h"); ok || b.Status().Services != before-1 {
		t.Fatal("unregistered record remains discoverable")
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#svc@h", Body: "late"}); !errors.Is(err, ErrUnknown) {
		t.Fatalf("removed inbox accepted a message: %v", err)
	}
	if err := b.Unregister("#svc@h", "owner@h"); !errors.Is(err, ErrUnknown) {
		t.Fatalf("second removal: %v", err)
	}
	// Exercise the serialized snapshot, not merely a shared in-memory map.
	data, err := json.Marshal(b.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var snapshot ports.Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	restored := New()
	restored.Restore(snapshot)
	for _, bus := range []*Bus{b, restored} {
		if owner, ok := bus.OwnerOf("#svc@h"); ok {
			t.Fatalf("a removed name still claims an owner: %q", owner)
		}
		// Whoever asks next gets it, previous owner or not. That is the
		// simplification: nothing is held back for a name nobody serves.
		if _, err := bus.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "stranger@h"}); err != nil {
			t.Fatalf("a removed name was still reserved: %v", err)
		}
		if err := bus.Unregister("#svc@h", "stranger@h"); err != nil {
			t.Fatalf("principal could not remove its own address: %v", err)
		}
	}
}

func TestUnregisterRefusesQueueAndEveryReader(t *testing.T) {
	b := newBusWith(t, "#svc@h")
	if _, err := b.Send(protocol.Envelope{From: "#svc@h", To: "#svc@h", Body: "keep"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("#svc@h", "#svc@h"); !errors.Is(err, ErrBusy) {
		t.Fatalf("queued message was discarded: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e, err := b.Consume(ctx, "#svc@h", "", "", false, false); err != nil || e.Body != "keep" {
		t.Fatalf("refusal lost queued message: %v %v", e, err)
	}
	for _, filtered := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { defer close(done); b.Consume(ctx, "#svc@h", "", "", filtered, false) }()
		waitForWaiters(t, b, "#svc@h", 1)
		err := b.Unregister("#svc@h", "#svc@h")
		cancel()
		<-done
		if !errors.Is(err, ErrBusy) {
			t.Fatalf("removed waiting reader (filtered=%v): %v", filtered, err)
		}
	}
	if err := b.Unregister("#svc@h", "#svc@h"); err != nil {
		t.Fatal(err)
	}
}

func TestUnregisterClearsConfigurationAndSubscriptions(t *testing.T) {
	b := newBusWith(t, "#svc@h")
	if _, err := b.Configure("#svc@h", "#svc@h", json.RawMessage(`{"private":true}`)); err != nil {
		t.Fatal(err)
	}
	provision(t, b, nil, protocol.Record{Name: "topic@h", Allow: []string{"*"}, Kind: protocol.KindPubSub})
	if _, err := b.Manage("topic@h", Management{Name: "topic@h", Subs: ptr([]string{"#svc@h"})}); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("#svc@h", "#svc@h"); err != nil {
		t.Fatal(err)
	}
	topic, _ := b.Lookup("#svc@h", "topic@h")
	if len(topic.Subs) != 0 {
		t.Fatal("subscription survived removal")
	}
	if _, ok := b.inboxes["#svc@h"]; ok {
		t.Fatal("inbox survived removal")
	}
	known(t, b, "keeper@h")
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "keeper@h"}); err != nil {
		t.Fatal(err)
	}
	if cfg, err := b.Config("#svc@h", "#svc@h"); err != nil || len(cfg) != 0 {
		t.Fatalf("configuration survived removal: %s %v", cfg, err)
	}
}

// Nothing but a User owns records, and a User is never removed, so removing a
// record never strands others: an Agent owns nothing and leaves freely, and
// its Owner's records stay the Owner's (docs/constitution.md#-registry-record).
func TestRemovingAnAgentStrandsNothing(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h")
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#child@h", Owner: "alice@h"}); err != nil {
		t.Fatal(err)
	}
	// Registered by the agent, so owned by alice.
	if _, err := b.Register(protocol.Record{Kind: protocol.KindQueue, Name: "made@h", Owner: "#child@h"}); err != nil {
		t.Fatal(err)
	}
	if owner, _ := b.OwnerOf("made@h"); owner != "alice@h" {
		t.Fatalf("a record an agent made is owned by %q", owner)
	}
	if err := b.Unregister("#child@h", "alice@h"); err != nil {
		t.Fatalf("an agent owning nothing could not leave: %v", err)
	}
	if owner, ok := b.OwnerOf("made@h"); !ok || owner != "alice@h" {
		t.Fatalf("the agent's removal changed what its owner owns: %q %v", owner, ok)
	}
}
