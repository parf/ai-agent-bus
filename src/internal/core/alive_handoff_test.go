package core

import (
	"context"
	"errors"
	"testing"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A message handed to a poll whose transport says its reader is gone, while its
// context is still live, is returned to the inbox and the poll gets an error:
// a success with an empty message would be requeued again as a phantom.
func TestHandoffToAGoneReaderWithALiveContextIsAnError(t *testing.T) {
	b := newBusWith(t, "#svc@h", "#peer@h")
	if _, err := b.Send(protocol.Envelope{From: "#peer@h", To: "#svc@h", Body: "keep"}); err != nil {
		t.Fatal(err)
	}
	e := readOne(t, b, "#svc@h")
	ctx := WithAlive(context.Background(), func() bool { return false })
	w := &waiter{ctx: ctx, alive: func() bool { return false }}
	got, err := b.handed("#svc@h", w, e, ctx)
	if err == nil || !errors.Is(err, context.Canceled) || got.ID != "" {
		t.Fatalf("handed %+v %v, want an error and no message", got, err)
	}
	if again := readOne(t, b, "#svc@h"); again.Body != "keep" {
		t.Fatalf("read %q, want the message back in the inbox once", again.Body)
	}
	b.mu.Lock()
	queued := len(b.inboxes["#svc@h"].queue)
	b.mu.Unlock()
	if queued != 0 {
		t.Fatalf("%d messages left queued, want none", queued)
	}
}

// A reply returned to the inbox and handed on to its caller's filtered waiter
// is delivered: the reservation it ended and got back ends again, instead of
// holding later messages on the same topic and tag until the old deadline.
func TestRequeueToAFilteredWaiterEndsTheReservation(t *testing.T) {
	f := newHeld(t)
	f.call(t, "#agent@h", "t1", "30s")
	f.reply(t, "#agent@h", "t1", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e, err := f.b.ConsumeAddressed(ctx, "#caller@h", "#caller@h", "call", "t1", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	w := &waiter{ctx: ctx, filtered: true, topic: "call", tag: "t1", ch: make(chan protocol.Envelope, 1)}
	f.b.mu.Lock()
	f.b.ensure("#caller@h").waiters = append(f.b.ensure("#caller@h").waiters, w)
	f.b.mu.Unlock()
	f.b.Requeue("#caller@h", e)
	<-w.ch
	f.b.mu.Lock()
	defer f.b.mu.Unlock()
	if n := len(f.b.ensure("#caller@h").holds); n != 0 {
		t.Fatalf("the filtered hand-off left %d reply reservations; later messages on the topic and tag stay held", n)
	}
}
