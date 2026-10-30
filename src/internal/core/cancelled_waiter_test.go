package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// goneWaiter registers a waiter whose poll was cancelled but which the daemon
// has not yet removed: what a reader that went away leaves behind until its
// handler notices.
func goneWaiter(t *testing.T, b *Bus, name string) *waiter {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := &waiter{ch: make(chan protocol.Envelope, 1), ctx: ctx}
	b.mu.Lock()
	in := b.ensure(name)
	in.waiters = append(in.waiters, w)
	b.mu.Unlock()
	return w
}

func readOne(t *testing.T, b *Bus, name string) protocol.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, err := b.Consume(ctx, name, "", "", false, false)
	if err != nil {
		t.Fatalf("nothing to read: %v", err)
	}
	return e
}

// A message is never handed to a waiter whose poll was cancelled: it stays in
// the inbox for the next reader.
func TestSendDoesNotHandAMessageToACancelledWaiter(t *testing.T) {
	b := newBusWith(t, "#svc@h", "#peer@h")
	w := goneWaiter(t, b, "#svc@h")
	if _, err := b.Send(protocol.Envelope{From: "#peer@h", To: "#svc@h", Body: "keepme"}); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-w.ch:
		t.Fatalf("handed %q to a cancelled waiter", e.Body)
	default:
	}
	if e := readOne(t, b, "#svc@h"); e.Body != "keepme" {
		t.Fatalf("read %q, want the queued message", e.Body)
	}
}

// A cancelled reader's leftover waiter is not a second reader: the next
// reader is admitted, not refused for it.
func TestCancelledWaiterDoesNotMakeTheNextReaderASecond(t *testing.T) {
	b := newBusWith(t, "#svc@h", "#peer@h")
	goneWaiter(t, b, "#svc@h")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := b.Consume(ctx, "#svc@h", "", "", false, false); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("the new reader got %v, want it admitted and left waiting", err)
	}
}

// What a poll was handed just as it was cancelled goes back to the front of
// the inbox, ahead of what came after it, rather than with the reader that is
// gone.
func TestMessageHandedToAPollCancelledMeanwhileGoesBackToTheFront(t *testing.T) {
	b := newBusWith(t, "#svc@h", "#peer@h")
	for _, body := range []string{"first", "second"} {
		if _, err := b.Send(protocol.Envelope{From: "#peer@h", To: "#svc@h", Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	first := readOne(t, b, "#svc@h")
	if first.Body != "first" {
		t.Fatalf("read %q", first.Body)
	}
	w := goneWaiter(t, b, "#svc@h")
	b.mu.Lock()
	b.ensure("#svc@h").waiters = nil // the waiter that received it is already off the list
	b.mu.Unlock()
	got, err := b.handed("#svc@h", w, first, w.ctx)
	if err == nil || got.Body != "" {
		t.Fatalf("handed %+v %v to a cancelled poll, want the message withheld", got, err)
	}
	if e := readOne(t, b, "#svc@h"); e.Body != "first" {
		t.Fatalf("read %q, want the returned message ahead of the later one", e.Body)
	}
	if e := readOne(t, b, "#svc@h"); e.Body != "second" {
		t.Fatalf("read %q, want the later one next", e.Body)
	}
}

// A call's reply taken by its caller's filtered read and then returned unread
// is held for that caller again: the unfiltered reader (a push adapter) must
// not get the caller's answer.
func TestRequeuedReplyKeepsItsReservationForTheCaller(t *testing.T) {
	f := newHeld(t)
	f.call(t, "#agent@h", "t1", "30s")
	f.reply(t, "#agent@h", "t1", "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, err := f.b.ConsumeAddressed(ctx, "#caller@h", "#caller@h", "call", "t1", "", true, false)
	if err != nil || e.Body != "answer" {
		t.Fatalf("the caller's filtered read: %+v %v", e, err)
	}
	f.b.Requeue("#caller@h", e) // the connection it was written to had closed
	short, cancelShort := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelShort()
	if got, err := f.b.Consume(short, "#caller@h", "", "", false, false); err == nil {
		t.Fatalf("the unfiltered reader took the caller's answer %q", got.Body)
	}
	again, err := f.b.ConsumeAddressed(ctx, "#caller@h", "#caller@h", "call", "t1", "", true, false)
	if err != nil || again.Body != "answer" {
		t.Fatalf("the caller's next filtered read: %+v %v", again, err)
	}
}

// What a filtered read skipped over stays ahead of the message it took when
// that goes back: arrival order is kept.
func TestRequeuePutsAMessageBackInArrivalOrder(t *testing.T) {
	b := newBusWith(t, "#svc@h", "#peer@h")
	for _, m := range []struct{ body, topic string }{{"a", "x"}, {"b", "y"}, {"c", "x"}} {
		if _, err := b.Send(protocol.Envelope{From: "#peer@h", To: "#svc@h", Body: m.body, Topic: m.topic}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	taken, err := b.ConsumeAddressed(ctx, "#svc@h", "#svc@h", "y", "", "", true, false)
	if err != nil || taken.Body != "b" {
		t.Fatalf("filtered read: %+v %v", taken, err)
	}
	b.Requeue("#svc@h", taken)
	for _, want := range []string{"a", "b", "c"} {
		if got := readOne(t, b, "#svc@h"); got.Body != want {
			t.Fatalf("read %q, want %q", got.Body, want)
		}
	}
}

// A poll whose transport says its reader is gone is not handed a message even
// though its context has not heard yet, and does not take a queued one.
func TestPollWhoseReaderIsGoneTakesNothing(t *testing.T) {
	b := newBusWith(t, "#svc@h", "#peer@h")
	if _, err := b.Send(protocol.Envelope{From: "#peer@h", To: "#svc@h", Body: "queued"}); err != nil {
		t.Fatal(err)
	}
	gone := WithAlive(context.Background(), func() bool { return false })
	if e, err := b.Consume(gone, "#svc@h", "", "", false, false); err == nil {
		t.Fatalf("a gone reader took %q from the queue", e.Body)
	}
	if e := readOne(t, b, "#svc@h"); e.Body != "queued" {
		t.Fatalf("read %q, want the message still queued", e.Body)
	}
}
