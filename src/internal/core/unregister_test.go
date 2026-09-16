package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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
		if owner, ok := bus.OwnerOf("svc@h"); ok {
			t.Fatalf("a removed name still claims an owner: %q", owner)
		}
		// Whoever asks next gets it, previous owner or not. That is the
		// simplification: nothing is held back for a name nobody serves.
		if _, err := bus.Register(protocol.Record{Name: "svc@h", Owner: "stranger@h"}); err != nil {
			t.Fatalf("a removed name was still reserved: %v", err)
		}
		if err := bus.Unregister("svc@h", "stranger@h"); err != nil {
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
	provision(t, b, protocol.Record{Name: "topic@h", Kind: protocol.KindTopic, Mode: protocol.ModePubSub})
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
	known(t, b, "keeper@h")
	if _, err := b.Register(protocol.Record{Name: "svc@h", Owner: "keeper@h"}); err != nil {
		t.Fatal(err)
	}
	if cfg, err := b.Config("svc@h", "svc@h"); err != nil || len(cfg) != 0 {
		t.Fatalf("configuration survived removal: %s %v", cfg, err)
	}
}

// Removing your own record while you still own others would leave each of them
// owned by a name the daemon no longer knows — the orphan the deletion rule is
// for (docs/01-identity.md#when-the-owner-is-gone), made by an ordinary call
// rather than by an old store. There is somebody here to tell, so it is
// refused like a queue that is not empty rather than cascading.
func TestUnregisteringDoesNotOrphanWhatItOwns(t *testing.T) {
	b := New()
	b.Administrator("admin@h")
	known(t, b, "alice@h")
	for _, r := range []protocol.Record{
		{Name: "child@h", Owner: "alice@h"},
		{Name: "other@h", Owner: "alice@h"},
	} {
		if _, err := b.Register(r); err != nil {
			t.Fatal(err)
		}
	}
	err := b.Unregister("alice@h", "alice@h")
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("a principal unregistered itself out from under what it owns: %v", err)
	}
	// The refusal says what is in the way, because the caller has to act on it.
	for _, want := range []string{"child@h", "other@h"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %s: %v", want, err)
		}
	}
	if b.Authenticate("alice@h") != nil {
		t.Fatal("the refused removal took the principal anyway")
	}
	// Clear what is in the way and the removal is allowed. A name that owns
	// nothing but itself was never the problem.
	for _, n := range []string{"child@h", "other@h"} {
		if err := b.Unregister(n, "alice@h"); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Unregister("alice@h", "alice@h"); err != nil {
		t.Fatalf("a principal owning only itself could not leave: %v", err)
	}
	// The same fact when somebody else owns the name's record. What the guard
	// weighs is what removal costs the name, not who held it: admin owning
	// alice's record does not make alice's services safe to strand.
	if _, err := b.Register(protocol.Record{Name: "carol@h", Owner: "admin@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "hers@h", Owner: "carol@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("carol@h", "admin@h"); !errors.Is(err, ErrBusy) {
		t.Fatalf("a record owned by somebody else was removed out from under what its name owns: %v", err)
	}

	// Positive control: a registered user keeps its services when its record
	// goes, because the user is still somebody the daemon knows. The guard is
	// about losing the last standing a name has, not about owning services.
	if _, err := b.SetUser("admin@h", protocol.User{Name: "dave@h"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "daves@h", Owner: "dave@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("dave@h", "dave@h"); err != nil {
		t.Fatalf("a registered user could not give up its record: %v", err)
	}
	if b.Authenticate("dave@h") != nil {
		t.Error("the user stopped being somebody when its record went")
	}

	// Positive control: owning a record is not the same as being one. A record
	// somebody else owns is removed without this guard ever applying.
	known(t, b, "bob@h")
	if _, err := b.Register(protocol.Record{Name: "theirs@h", Owner: "bob@h"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Unregister("theirs@h", "bob@h"); err != nil {
		t.Fatalf("owned record could not be removed: %v", err)
	}
}
