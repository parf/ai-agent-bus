package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	if _, err := b.Send(protocol.Envelope{From: "peer@h", To: "ghost@h"}); !errors.Is(err, ErrUnknown) {
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

// Nothing dead stays reachable through a queue or a waiter list. Shortening
// a slice leaves the vacated slot pointing at what was removed, so a drained
// inbox went on holding every body it handed out, and a served waiter's
// channel stayed rooted. The backing array is deliberately kept; only the
// contents of the slots past len must be gone.
//
// Bodies are large here on purpose: this is about what memory is held.
func bigBody(i int) string { return fmt.Sprintf("%d-%s", i, strings.Repeat("x", 4096)) }

func heldEnvelopes(q []protocol.Envelope) int {
	held := 0
	for _, e := range q[:cap(q)] {
		if e.Body != "" || e.ID != "" {
			held++
		}
	}
	return held
}

func TestConsumedMessagesAreReleased(t *testing.T) {
	for _, tc := range []struct {
		name  string
		order func(n int) []int // the tags to consume, in order
	}{
		{"fifo", func(n int) []int {
			out := make([]int, n)
			for i := range out {
				out[i] = i
			}
			return out
		}},
		{"back to front", func(n int) []int {
			out := make([]int, n)
			for i := range out {
				out[i] = n - 1 - i
			}
			return out
		}},
		{"middle first", func(n int) []int {
			out := []int{n / 2, n - 1, 0}
			for i := 0; i < n; i++ {
				if i != n/2 && i != n-1 && i != 0 {
					out = append(out, i)
				}
			}
			return out
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := New()
			if _, err := b.Register(protocol.Record{Name: "sink@h", Owner: "sink@h"}); err != nil {
				t.Fatal(err)
			}
			const n = 100
			for i := 0; i < n; i++ {
				if _, err := b.Send(protocol.Envelope{
					To: "sink@h", From: "s@h", Tag: fmt.Sprint(i), Body: bigBody(i),
				}); err != nil {
					t.Fatal(err)
				}
			}
			for _, tag := range tc.order(n) {
				if _, err := b.Consume(context.Background(), "sink@h", "", fmt.Sprint(tag), true); err != nil {
					t.Fatalf("tag %d: %v", tag, err)
				}
			}
			b.mu.Lock()
			defer b.mu.Unlock()
			q := b.inboxes["sink@h"].queue
			if len(q) != 0 {
				t.Fatalf("queue is not empty: %d", len(q))
			}
			if held := heldEnvelopes(q); held != 0 {
				t.Fatalf("a drained inbox still holds %d consumed envelopes", held)
			}
		})
	}
}

// What a ring drops is dropped, not kept in the slack behind the queue.
func TestRingDropsAreReleased(t *testing.T) {
	b := New()
	if _, err := b.Register(protocol.Record{
		Name: "ringy@h", Owner: "ringy@h", Full: protocol.OverflowRing,
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxQueue+50; i++ {
		if _, err := b.Send(protocol.Envelope{To: "ringy@h", From: "s@h", Body: bigBody(i)}); err != nil {
			t.Fatal(err)
		}
	}
	b.mu.Lock()
	q := b.inboxes["ringy@h"].queue
	if len(q) != maxQueue {
		b.mu.Unlock()
		t.Fatalf("queue holds %d, want %d", len(q), maxQueue)
	}
	// The queue has stopped growing, so this is the backing array the drops
	// will happen in. Taking it now is the only way to see the slots a
	// head-advancing `q = q[1:]` would leave behind its own start.
	base := q[:cap(q)]
	b.mu.Unlock()

	const dropped = 50
	for i := 0; i < dropped; i++ {
		if _, err := b.Send(protocol.Envelope{To: "ringy@h", From: "s@h", Body: bigBody(9000 + i)}); err != nil {
			t.Fatal(err)
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	q = b.inboxes["ringy@h"].queue
	if len(q) != maxQueue {
		t.Fatalf("after the drops the queue holds %d, want %d", len(q), maxQueue)
	}
	// A full ring does not grow, so the only thing that can move the head is
	// dropping by re-slicing — and everything the head marches past stays
	// reachable through the array for as long as the inbox lives.
	if &base[0] != &q[0] {
		t.Fatal("the queue head moved: dropped messages are left behind it")
	}
	live := map[string]bool{}
	for _, e := range q {
		live[e.Body] = true
	}
	for i, e := range base {
		if e.Body != "" && !live[e.Body] {
			t.Fatalf("slot %d still holds a dropped message", i)
		}
	}
	if q[0].Body == "" {
		t.Fatal("a live message was cleared")
	}
}

// A waiter that has been served or has given up must not stay reachable.
func TestServedWaitersAreReleased(t *testing.T) {
	b := New()
	if _, err := b.Register(protocol.Record{Name: "w@h", Owner: "w@h"}); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := b.Consume(context.Background(), "w@h", "t", "g", true); err != nil {
			t.Error(err)
		}
	}()
	for i := 0; i < 200; i++ {
		b.mu.Lock()
		n := len(b.inboxes["w@h"].waiters)
		b.mu.Unlock()
		if n == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := b.Send(protocol.Envelope{To: "w@h", From: "s@h", Topic: "t", Tag: "g", Body: "hi"}); err != nil {
		t.Fatal(err)
	}
	<-done
	b.mu.Lock()
	defer b.mu.Unlock()
	ws := b.inboxes["w@h"].waiters
	if len(ws) != 0 {
		t.Fatalf("%d waiters left", len(ws))
	}
	for _, w := range ws[:cap(ws)] {
		if w != nil {
			t.Fatal("a served waiter is still reachable behind the list")
		}
	}
}

// The bus stores one spelling of a configuration, so reformatting a file does
// not look like changing it: the digest a query gets is over what is stored.
func TestAConfigurationIsStoredCompacted(t *testing.T) {
	b := New()
	spaced, err := b.Configure("svc@h", "svc@h", []byte("{ \"k\" : \"v\" }\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(spaced.Config); got != `{"k":"v"}` {
		t.Fatalf("stored %q", got)
	}
	compact, err := b.Configure("other@h", "other@h", []byte(`{"k":"v"}`))
	if err != nil {
		t.Fatal(err)
	}
	if a, c := spaced.Public().ConfigSHA, compact.Public().ConfigSHA; a != c {
		t.Fatalf("whitespace changed the digest:\n%s\n%s", a, c)
	}
}
