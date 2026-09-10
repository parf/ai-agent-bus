package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func newBusWith(t *testing.T, names ...string) *Bus {
	t.Helper()
	b := New()
	for _, n := range names {
		if _, err := b.Register(protocol.Record{Name: n, Kind: "agent"}); err != nil {
			t.Fatalf("register %s: %v", n, err)
		}
	}
	return b
}

// A waiter that asked for this topic+tag must be served ahead of the
// unfiltered reader, even though the reader blocked first. Without this a
// `call` loses its reply to whatever else is reading the inbox.
func TestFilteredWaiterBeatsEarlierUnfilteredReader(t *testing.T) {
	b := newBusWith(t, "svc@h", "peer@h")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	unfiltered := make(chan protocol.Envelope, 1)
	go func() {
		e, err := b.Consume(ctx, "svc@h", "", "", false)
		if err == nil {
			unfiltered <- e
		}
	}()
	waitForWaiters(t, b, "svc@h", 1)

	filtered := make(chan protocol.Envelope, 1)
	go func() {
		e, err := b.Consume(ctx, "svc@h", "t", "g", true)
		if err == nil {
			filtered <- e
		}
	}()
	waitForWaiters(t, b, "svc@h", 2)

	if _, err := b.Send(protocol.Envelope{From: "peer@h", To: "svc@h", Topic: "t", Tag: "g", Body: "reply"}); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-filtered:
		if e.Body != "reply" {
			t.Fatalf("wrong body: %q", e.Body)
		}
	case e := <-unfiltered:
		t.Fatalf("the unfiltered reader stole a filtered waiter's message: %q", e.Body)
	case <-time.After(time.Second):
		t.Fatal("nobody got the message")
	}
}

// A send that reports success must not vanish because the waiter's deadline
// fired at the same moment: either Consume returns it, or it is still queued.
func TestCancelDoesNotSwallowAMessage(t *testing.T) {
	for i := 0; i < 300; i++ {
		b := newBusWith(t, "svc@h", "peer@h")
		ctx, cancel := context.WithCancel(context.Background())

		got := make(chan protocol.Envelope, 1)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e, err := b.Consume(ctx, "svc@h", "", "", false); err == nil {
				got <- e
			}
		}()
		waitForWaiters(t, b, "svc@h", 1)

		go cancel()
		if _, err := b.Send(protocol.Envelope{From: "peer@h", To: "svc@h", Body: "keepme"}); err != nil {
			t.Fatal(err)
		}
		wg.Wait()

		select {
		case <-got: // delivered to the caller
		default:
			// not delivered, so it must still be waiting in the queue
			read, cancel2 := context.WithTimeout(context.Background(), time.Second)
			e, err := b.Consume(read, "svc@h", "", "", false)
			cancel2()
			if err != nil || e.Body != "keepme" {
				t.Fatalf("iteration %d: message lost between cancel and send (err %v)", i, err)
			}
		}
		cancel()
	}
}

// " x@y " and "x@y" are the same inbox, or a registration lands somewhere a
// send can never reach.
func TestNamesAreCanonical(t *testing.T) {
	b := New()
	if _, err := b.Register(protocol.Record{Name: "  svc@h  ", Kind: "agent"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Name: "peer@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: " peer@h", To: "svc@h ", Body: "hi"}); err != nil {
		t.Fatalf("send to the registered name failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, err := b.Consume(ctx, "svc@h", "", "", false)
	if err != nil || e.Body != "hi" {
		t.Fatalf("consume: %v %q", err, e.Body)
	}
	if e.From != "peer@h" {
		t.Fatalf("sender not canonical: %q", e.From)
	}
}

// One outstanding unfiltered read per inbox; a filtered waiter is fine beside it.
func TestSecondUnfilteredReaderRefused(t *testing.T) {
	b := newBusWith(t, "svc@h")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go b.Consume(ctx, "svc@h", "", "", false)
	waitForWaiters(t, b, "svc@h", 1)

	if _, err := b.Consume(ctx, "svc@h", "", "", false); err != ErrTwoReads {
		t.Fatalf("want ErrTwoReads, got %v", err)
	}
	done := make(chan error, 1)
	go func() { _, err := b.Consume(ctx, "svc@h", "t", "g", true); done <- err }()
	waitForWaiters(t, b, "svc@h", 2)
	cancel()
	if err := <-done; err == ErrTwoReads {
		t.Fatal("a filtered waiter must be allowed alongside the reader")
	}
}

func TestSendToUnknownName(t *testing.T) {
	b := newBusWith(t, "peer@h")
	if _, err := b.Send(protocol.Envelope{From: "peer@h", To: "ghost@h"}); err != ErrUnknown {
		t.Fatalf("want ErrUnknown, got %v", err)
	}
	if _, err := b.Send(protocol.Envelope{From: "peer@h", To: "no-realm"}); err == nil {
		t.Fatal("a name without a realm must be refused")
	}
}

func waitForWaiters(t *testing.T, b *Bus, name string, n int) {
	t.Helper()
	for i := 0; i < 200; i++ {
		b.mu.Lock()
		got := len(b.ensure(name).waiters)
		b.mu.Unlock()
		if got >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("waited for %d waiters on %s, never arrived", n, name)
}

// A queue topic is an inbox with a name: anyone may send to it, and the
// consumer that was down still finds the message.
func TestQueueTopicHoldsAMessageForAConsumerThatWasDown(t *testing.T) {
	b := New()
	mustRegister(t, b, protocol.Record{Name: "jobs@srv", Kind: protocol.KindTopic, Mode: protocol.ModeQueue, Owner: "a@srv"})
	mustRegister(t, b, protocol.Record{Name: "pub@srv", Owner: "pub@srv"})
	if _, err := b.Send(protocol.Envelope{From: "pub@srv", To: "jobs@srv", Topic: "jobs@srv", Body: "work"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, err := b.Consume(ctx, "jobs@srv", "", "", false)
	if err != nil || e.Body != "work" {
		t.Fatalf("reading the topic afterwards: %v %+v", err, e)
	}
}

// Fan-out is MVP, and PoC says so instead of inventing a second meaning of
// subscription. See docs/12-stages.md#poc.
func TestPublishingToAPubSubTopicSaysMVP(t *testing.T) {
	b := New()
	mustRegister(t, b, protocol.Record{Name: "news@srv", Kind: protocol.KindTopic, Mode: protocol.ModePubSub, Owner: "a@srv"})
	mustRegister(t, b, protocol.Record{Name: "pub@srv", Owner: "pub@srv"})
	_, err := b.Send(protocol.Envelope{From: "pub@srv", To: "news@srv", Body: "x"})
	if !errors.Is(err, ErrNotYet) {
		t.Fatalf("want ErrNotYet, got %v", err)
	}
}

// A receipt rides the same topic and tag as the message it is about, so the
// caller's filtered wait sees it. See docs/04-messaging.md#receipts.
func TestAReceiptReachesTheCallersFilteredWait(t *testing.T) {
	b := New()
	mustRegister(t, b, protocol.Record{Name: "caller@srv", Owner: "caller@srv"})
	if _, err := b.Send(protocol.Envelope{
		From: "caller@srv", To: "caller@srv", Topic: "call", Tag: "t1", Receipt: "ack", Re: "abc",
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, err := b.Consume(ctx, "caller@srv", "call", "t1", true)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if e.Receipt != "ack" || e.Re != "abc" {
		t.Fatalf("the receipt lost its meaning: %+v", e)
	}
}

func mustRegister(t *testing.T, b *Bus, r protocol.Record) {
	t.Helper()
	if _, err := b.Register(r); err != nil {
		t.Fatalf("register %s: %v", r.Name, err)
	}
}
