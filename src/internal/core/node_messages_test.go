package core

import (
	"context"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestNodeMessageWindowsSurviveRemovalAndUseSampleTimes(t *testing.T) {
	b := New()
	b.Administrator("owner@h")
	mustRegister(t, b, protocol.Record{Name: "private@h", Owner: "owner@h", Allow: []string{"owner@h"}})
	start := time.Now().Add(-62 * time.Minute)
	send := func(n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "private@h", Body: "counted"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	b.SampleActivity(start)
	send(2)
	b.SampleActivity(start.Add(56 * time.Minute))
	send(3)
	b.SampleActivity(start.Add(60 * time.Minute))
	send(4)
	now := start.Add(61*time.Minute + 30*time.Second)
	got := b.NodeMessages(now)
	for i, want := range []struct {
		in   int
		span string
	}{{4, "1m30s"}, {7, "5m30s"}, {9, "1h1m30s"}} {
		if got[i].Window != []string{"1m", "5m", "60m"}[i] || !got[i].Available || got[i].Accepted != want.in || got[i].Dequeued != 0 || got[i].Observed != want.span {
			t.Fatalf("window %d: %+v", i, got[i])
		}
	}
	for i := 0; i < 9; i++ {
		if _, err := b.Consume(context.Background(), "private@h", "", "", false, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Unregister("private@h", "owner@h"); err != nil {
		t.Fatal(err)
	}
	mustRegister(t, b, protocol.Record{Name: "private@h", Owner: "owner@h"})
	send(1)
	got = b.NodeMessages(now)
	if got[0].Accepted != 5 || got[0].Dequeued != 9 || got[2].Accepted != 10 {
		t.Fatalf("deletion/reuse erased traffic or dequeues were bounded by arrivals: %+v", got)
	}
	// Snapshot restores inbox counters, but never carries these process windows.
	clone := New()
	clone.Restore(b.Snapshot())
	for _, w := range clone.NodeMessages(now) {
		if w.Available {
			t.Fatal("new run invented history")
		}
	}
	clone.SampleActivity(now)
	if _, err := clone.Consume(context.Background(), "private@h", "", "", false, false); err != nil {
		t.Fatal(err)
	}
	for _, w := range clone.NodeMessages(now.Add(5 * time.Second)) {
		if !w.Available || w.Observed != "5s" || w.Accepted != 0 || w.Dequeued != 1 {
			t.Fatalf("restored counters counted as new traffic or partial history concealed: %+v", w)
		}
	}
}

func TestNodeMessagesCountInboxCopiesAndDirectDelivery(t *testing.T) {
	b := New()
	b.Administrator("owner@h")
	for _, name := range []string{"a@h", "b@h"} {
		mustRegister(t, b, protocol.Record{Name: name, Owner: "owner@h", Bound: 1})
	}
	mustRegister(t, b, protocol.Record{Name: "news@h", Owner: "owner@h", Kind: protocol.KindTopic, Mode: protocol.ModePubSub})
	at := time.Now()
	b.SampleActivity(at)
	publish := func() {
		t.Helper()
		if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "news@h", Body: "publication"}); err != nil {
			t.Fatal(err)
		}
	}
	publish()
	if got := b.NodeMessages(at.Add(time.Second))[0]; got.Accepted != 0 {
		t.Fatalf("a publication with no subscribers counted as inbox delivery: %+v", got)
	}
	for _, name := range []string{"a@h", "b@h"} {
		if _, err := b.Subscribe(name, "news@h", true); err != nil {
			t.Fatal(err)
		}
	}
	publish()
	publish() // the second publication finds both strict inboxes full
	if got := b.NodeMessages(at.Add(time.Second))[0]; got.Accepted != 2 || got.Dequeued != 0 {
		t.Fatalf("copies/refused copies miscounted: %+v", got)
	}
	for _, name := range []string{"a@h", "b@h"} {
		if _, err := b.Consume(context.Background(), name, "", "", false, false); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan protocol.Envelope, 1)
	go func() { e, _ := b.Consume(ctx, "a@h", "", "", false, false); result <- e }()
	waitForWaiters(t, b, "a@h", 1)
	if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "a@h", Body: "straight through"}); err != nil {
		t.Fatal(err)
	}
	if e := <-result; e.Body != "straight through" {
		t.Fatal("direct delivery did not reach reader")
	}
	if got := b.NodeMessages(at.Add(time.Second))[0]; got.Accepted != 3 || got.Dequeued != 3 {
		t.Fatalf("straight-through counters differ: %+v", got)
	}
}
