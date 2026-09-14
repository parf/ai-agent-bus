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

func TestUnregisterPreservesIdentityAcrossRestart(t *testing.T) {
	b := New()
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "owner@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("svc@h", "stranger@h"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("stranger removed record: %v", err)
	}
	if err := b.Unregister(" SVC@H ", " OWNER@H "); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.Lookup("owner@h", "svc@h"); ok || b.Status().Services != 0 {
		t.Fatal("unregistered record remains discoverable")
	}
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "svc@h", Body: "late"}); !errors.Is(err, ErrUnknown) {
		t.Fatalf("removed inbox accepted a message: %v", err)
	}
	if err := b.Unregister("svc@h", "owner@h"); !errors.Is(err, ErrUnknown) {
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
		if owner, ok := bus.OwnerOf("svc@h"); !ok || owner != "owner@h" {
			t.Fatal("unregister lost credential ownership")
		}
		if _, err := bus.Register(protocol.Record{Name: "svc@h", Owner: "stranger@h"}); !errors.Is(err, ErrNotOwner) {
			t.Fatalf("registration stole retired identity: %v", err)
		}
		if _, err := bus.Configure("svc@h", "stranger@h", json.RawMessage(`{}`)); !errors.Is(err, ErrNotOwner) {
			t.Fatalf("configuration stole retired identity: %v", err)
		}
		if _, err := bus.RegisterNew(protocol.Record{Name: "svc@h", Owner: "owner@h"}); err != nil {
			t.Fatalf("owner could not reclaim address: %v", err)
		}
		if err := bus.Unregister("svc@h", "svc@h"); err != nil {
			t.Fatalf("principal could not remove its own address: %v", err)
		}
	}
}

func TestUnregisterRefusesQueueAndEveryReader(t *testing.T) {
	b := newBusWith(t, "svc@h")
	if _, err := b.Send(protocol.Envelope{From: "svc@h", To: "svc@h", Body: "keep"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("svc@h", "svc@h"); !errors.Is(err, ErrBusy) {
		t.Fatalf("queued message was discarded: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e, err := b.Consume(ctx, "svc@h", "", "", false, false); err != nil || e.Body != "keep" {
		t.Fatalf("refusal lost queued message: %v %v", e, err)
	}
	for _, filtered := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { defer close(done); b.Consume(ctx, "svc@h", "", "", filtered, false) }()
		waitForWaiters(t, b, "svc@h", 1)
		err := b.Unregister("svc@h", "svc@h")
		cancel()
		<-done
		if !errors.Is(err, ErrBusy) {
			t.Fatalf("removed waiting reader (filtered=%v): %v", filtered, err)
		}
	}
	if err := b.Unregister("svc@h", "svc@h"); err != nil {
		t.Fatal(err)
	}
}

func TestUnregisterClearsConfigurationAndSubscriptions(t *testing.T) {
	b := newBusWith(t, "svc@h")
	if _, err := b.Configure("svc@h", "svc@h", json.RawMessage(`{"private":true}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "topic@h", Kind: protocol.KindTopic, Mode: protocol.ModePubSub}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Subscribe("svc@h", "topic@h", true); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("svc@h", "svc@h"); err != nil {
		t.Fatal(err)
	}
	topic, _ := b.Lookup("svc@h", "topic@h")
	if len(topic.Subs) != 0 {
		t.Fatal("subscription survived removal")
	}
	if _, ok := b.inboxes["svc@h"]; ok {
		t.Fatal("inbox survived removal")
	}
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "svc@h"}); err != nil {
		t.Fatal(err)
	}
	if cfg, err := b.Config("svc@h", "svc@h"); err != nil || len(cfg) != 0 {
		t.Fatalf("configuration survived removal: %s %v", cfg, err)
	}
}
