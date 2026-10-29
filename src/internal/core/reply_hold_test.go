package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A call names its own inbox as the reply route, and a push adapter reads that
// inbox unfiltered. The bus keeps the answer for the caller's filtered read
// (docs/04-messaging.md#request-and-reply); these cover each way it could
// reach the adapter instead.

type heldFixture struct {
	b *Bus
}

func newHeld(t *testing.T) heldFixture {
	return heldFixture{newBusWith(t, "#caller@h", "#agent@h", "#other@h")}
}

// call sends as the caller with a wait and the caller itself as the route.
func (f heldFixture) call(t *testing.T, to, tag, wait string) {
	t.Helper()
	_, err := f.b.Send(protocol.Envelope{From: "#caller@h", To: to, Topic: "call", Tag: tag, Wait: wait,
		ReplyTo: &protocol.ReplyTo{Name: "#caller@h", Topic: "call", Tag: tag}})
	if err != nil {
		t.Fatal(err)
	}
}

func (f heldFixture) reply(t *testing.T, from, tag, receipt string) {
	t.Helper()
	_, err := f.b.Send(protocol.Envelope{From: from, To: "#caller@h", Topic: "call", Tag: tag, Receipt: receipt, Body: "answer"})
	if err != nil {
		t.Fatal(err)
	}
}

// face starts the unfiltered reader and returns what it gets; it is a waiter
// before it returns.
func (f heldFixture) face(t *testing.T, ctx context.Context) <-chan protocol.Envelope {
	t.Helper()
	got := make(chan protocol.Envelope, 1)
	go func() {
		e, err := f.b.Consume(ctx, "#caller@h", "", "", false, false)
		if err == nil {
			got <- e
		}
	}()
	f.waiters(t, 1)
	return got
}

func (f heldFixture) waiters(t *testing.T, n int) {
	t.Helper()
	for deadline := time.Now().Add(time.Second); ; {
		f.b.mu.Lock()
		have := len(f.b.inboxes["#caller@h"].waiters)
		f.b.mu.Unlock()
		if have == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d waiters, want %d", have, n)
		}
		time.Sleep(time.Millisecond)
	}
}

func (f heldFixture) takeFiltered(tag string) (protocol.Envelope, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	e, err := f.b.Consume(ctx, "#caller@h", "call", tag, true, false)
	return e, err == nil
}

func (f heldFixture) none(t *testing.T, got <-chan protocol.Envelope, why string) {
	t.Helper()
	select {
	case e := <-got:
		t.Fatalf("%s: the unfiltered reader took %q", why, e.Body)
	case <-time.After(50 * time.Millisecond):
	}
}

// The answer is sent before any filtered read exists: the caller's own read
// must still get it, not the adapter's.
func TestReplyHeldForTheCallersFilteredReadNotTheAdapter(t *testing.T) {
	f := newHeld(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got := f.face(t, ctx)
	f.call(t, "#agent@h", "t1", "2s")
	f.reply(t, "#agent@h", "t1", "")
	f.none(t, got, "answer in the gap after the send")
	if e, ok := f.takeFiltered("t1"); !ok || e.Body != "answer" {
		t.Fatalf("the caller's filtered read got %q, %v; want the answer", e.Body, ok)
	}
}

// A receipt ends a filtered wait; the answer behind it goes through the same
// gap and is kept as well.
func TestReceiptAndAnswerAreBothHeld(t *testing.T) {
	f := newHeld(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got := f.face(t, ctx)
	f.call(t, "#agent@h", "t2", "2s")
	f.reply(t, "#agent@h", "t2", "ack")
	f.none(t, got, "receipt")
	if e, ok := f.takeFiltered("t2"); !ok || e.Receipt != "ack" {
		t.Fatalf("first filtered read got %+v, %v; want the ack", e, ok)
	}
	f.reply(t, "#agent@h", "t2", "")
	f.none(t, got, "answer after a receipt")
	if e, ok := f.takeFiltered("t2"); !ok || e.Body != "answer" {
		t.Fatalf("second filtered read got %+v, %v; want the answer", e, ok)
	}
}

// The first message that is not a receipt ends the hold: a second reply, or a
// duplicate, belongs to the adapter.
func TestASecondReplyFlowsToTheAdapter(t *testing.T) {
	f := newHeld(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got := f.face(t, ctx)
	f.call(t, "#agent@h", "t3", "2s")
	f.reply(t, "#agent@h", "t3", "")
	if _, ok := f.takeFiltered("t3"); !ok {
		t.Fatal("the answer was not taken")
	}
	f.reply(t, "#agent@h", "t3", "")
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("a duplicate reply after the answer was held")
	}
}

// Held means held back, not dropped: when the reservation ends the blocked
// adapter gets the reply with no further send.
func TestHeldReplyReachesTheAdapterWhenTheHoldEnds(t *testing.T) {
	f := newHeld(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got := f.face(t, ctx)
	f.call(t, "#agent@h", "t4", "150ms")
	f.reply(t, "#agent@h", "t4", "")
	f.none(t, got, "inside the wait")
	select {
	case e := <-got:
		if e.Body != "answer" {
			t.Fatalf("the adapter got %q", e.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the held reply never reached the blocked adapter")
	}
}

// A reply already queued is held on the queue scan too: an adapter that starts
// its read afterwards must not take it.
func TestQueuedHeldReplyIsSkippedByAnUnfilteredRead(t *testing.T) {
	f := newHeld(t)
	f.call(t, "#agent@h", "t5", "2s")
	f.reply(t, "#agent@h", "t5", "")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if e, err := f.b.Consume(ctx, "#caller@h", "", "", false, false); err == nil {
		t.Fatalf("the unfiltered read took the held reply %q from the queue", e.Body)
	}
	if e, ok := f.takeFiltered("t5"); !ok || e.Body != "answer" {
		t.Fatalf("the filtered read got %q, %v", e.Body, ok)
	}
}

// Only the reply to this request is held.
func TestOnlyTheRepliesToThisRequestAreHeld(t *testing.T) {
	cases := []struct {
		name string
		send func(t *testing.T, f heldFixture)
	}{
		{"another sender on the same topic and tag", func(t *testing.T, f heldFixture) {
			f.reply(t, "#other@h", "t6", "")
		}},
		{"the same sender and tag, another topic", func(t *testing.T, f heldFixture) {
			if _, err := f.b.Send(protocol.Envelope{From: "#agent@h", To: "#caller@h", Topic: "other", Tag: "t6", Body: "x"}); err != nil {
				t.Fatal(err)
			}
		}},
		{"the same sender, another tag", func(t *testing.T, f heldFixture) {
			f.reply(t, "#agent@h", "t6-other", "")
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newHeld(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got := f.face(t, ctx)
			f.call(t, "#agent@h", "t6", "2s")
			c.send(t, f)
			select {
			case <-got:
			case <-time.After(time.Second):
				t.Fatal("a message that is not the reply was held")
			}
		})
	}
}

// A route to a third party, or no wait, reserves nothing: the sender is not
// the one reading the answer.
func TestNoHoldUnlessTheCallerIsTheRoute(t *testing.T) {
	sends := map[string]protocol.Envelope{
		"a third party's route": {From: "#caller@h", To: "#agent@h", Topic: "call", Tag: "t7", Wait: "2s",
			ReplyTo: &protocol.ReplyTo{Name: "#other@h", Topic: "call", Tag: "t7"}},
		"no route": {From: "#caller@h", To: "#agent@h", Topic: "call", Tag: "t7", Wait: "2s"},
		"no wait": {From: "#caller@h", To: "#agent@h", Topic: "call", Tag: "t7",
			ReplyTo: &protocol.ReplyTo{Name: "#caller@h", Topic: "call", Tag: "t7"}},
	}
	for name, e := range sends {
		t.Run(name, func(t *testing.T) {
			f := newHeld(t)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got := f.face(t, ctx)
			if _, err := f.b.Send(e); err != nil {
				t.Fatal(err)
			}
			f.reply(t, "#agent@h", "t7", "")
			select {
			case <-got:
			case <-time.After(time.Second):
				t.Fatal("a reply was held without a reservation")
			}
		})
	}
}

// Past the cap a send behaves as it always did: accepted, and not held.
func TestHoldsPerInboxAreBounded(t *testing.T) {
	f := newHeld(t)
	for i := 0; i < maxHolds; i++ {
		f.call(t, "#agent@h", fmt.Sprintf("c%d", i), "2s")
	}
	f.call(t, "#agent@h", "over", "2s")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got := f.face(t, ctx)
	f.reply(t, "#agent@h", "over", "")
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("a reservation past the cap was made")
	}
}

// A request to a name that forwards is answered by the name it was forwarded
// to, and that answer is held too.
func TestAnswerFromTheForwardedToNameIsHeld(t *testing.T) {
	b := forwardFixture(t)
	if _, err := b.Manage("alice@h", Management{Name: "#a@h", Subs: &[]string{"#b@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Manage("bob@h", Management{Name: "#b@h", Allow: &[]string{"#a@h", "sender@h"}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got := make(chan protocol.Envelope, 1)
	go func() {
		if e, err := b.Consume(ctx, "sender@h", "", "", false, false); err == nil {
			got <- e
		}
	}()
	for deadline := time.Now().Add(time.Second); ; time.Sleep(time.Millisecond) {
		b.mu.Lock()
		n := len(b.inboxes["sender@h"].waiters)
		b.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the unfiltered reader did not block")
		}
	}
	if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "#a@h", Topic: "call", Tag: "fw", Wait: "2s",
		ReplyTo: &protocol.ReplyTo{Name: "sender@h", Topic: "call", Tag: "fw"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "#b@h", To: "sender@h", Topic: "call", Tag: "fw", Body: "answer"}); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-got:
		t.Fatalf("the unfiltered reader took the forwarded answer %q", e.Body)
	case <-time.After(50 * time.Millisecond):
	}
	c, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel2()
	if e, err := b.Consume(c, "sender@h", "call", "fw", true, false); err != nil || e.Body != "answer" {
		t.Fatalf("the filtered read got %q, %v", e.Body, err)
	}
}

// Two readers that share the inbox are both blocked and the caller has gone:
// the held reply reaches exactly one of them at the deadline.
func TestHeldReplyGoesToOneOfSeveralSharedReaders(t *testing.T) {
	f := newHeld(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got := make(chan protocol.Envelope, 2)
	for i := 0; i < 2; i++ {
		go func() {
			if e, err := f.b.Consume(ctx, "#caller@h", "", "", false, true); err == nil {
				got <- e
			}
		}()
	}
	f.waiters(t, 2)
	f.call(t, "#agent@h", "t9", "150ms")
	f.reply(t, "#agent@h", "t9", "")
	select {
	case <-got:
		t.Fatal("a shared reader took the held reply inside the wait")
	case <-time.After(50 * time.Millisecond):
	}
	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("no shared reader got the reply at the deadline")
	}
	select {
	case e := <-got:
		t.Fatalf("a second reader also got %q", e.Body)
	case <-time.After(100 * time.Millisecond):
	}
}
