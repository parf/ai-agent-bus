package core

import (
	"context"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func TestActivityIsBoundedAndFiltered(t *testing.T) {
	b := New()
	b.Administrator("admin@h")
	b.Register(protocol.Record{Name: "visible@h", Owner: "alice@h", Allow: []string{"alice@h"}, Bound: 1, Full: "ring"})
	b.Register(protocol.Record{Name: "hidden@h", Owner: "bob@h", Allow: []string{"bob@h"}})
	start := time.Now().Add(-2 * time.Minute)
	b.SampleActivity(start)
	b.Send(protocol.Envelope{From: "alice@h", To: "visible@h", Body: "first"})
	b.Send(protocol.Envelope{From: "alice@h", To: "visible@h", Body: "second"})
	b.Send(protocol.Envelope{From: "bob@h", To: "hidden@h", Body: "secret"})
	b.Consume(context.Background(), "visible@h", "", "", false, false)
	b.Send(protocol.Envelope{From: "alice@h", To: "visible@h", Body: "expires", TTL: "1ns"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b.Consume(ctx, "visible@h", "", "", false, false)
	b.RecordRefusal("VISIBLE@h")
	b.Refuse("acl")
	b.SampleActivity(start.Add(time.Minute))
	points, err := b.Activity("alice@h", "visible@h")
	if err != nil {
		t.Fatal(err)
	}
	if len(points) < 1 || points[0].In != 3 || points[0].Out != 1 || points[0].Dropped != 1 || points[0].Refused != 1 || points[0].Expired != 1 {
		t.Fatalf("wrong activity: %+v", points)
	}
	all, _ := b.Activity("alice@h", "")
	if all[0].In != 3 {
		t.Fatal("hidden service leaked into aggregate")
	}
	if _, err := b.Activity("alice@h", "hidden@h"); err != ErrUnknown {
		t.Fatal("forbidden graph returned")
	}
	for i := 0; i < 100; i++ {
		b.SampleActivity(start.Add(time.Duration(i+2)*time.Millisecond + time.Minute))
	}
	if len(b.activity) != activityKept {
		t.Fatalf("unbounded history: %d", len(b.activity))
	}
	clone := New()
	clone.Restore(b.Snapshot())
	if len(clone.activity) != 0 {
		t.Fatal("history unexpectedly survives restart")
	}
}
