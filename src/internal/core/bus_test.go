package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

func newBusWith(t *testing.T, names ...string) *Bus {
	t.Helper()
	b := New()
	known(t, b, names...)
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
		e, err := b.Consume(ctx, "svc@h", "", "", false, false)
		if err == nil {
			unfiltered <- e
		}
	}()
	waitForWaiters(t, b, "svc@h", 1)

	filtered := make(chan protocol.Envelope, 1)
	go func() {
		e, err := b.Consume(ctx, "svc@h", "t", "g", true, false)
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
			if e, err := b.Consume(ctx, "svc@h", "", "", false, false); err == nil {
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
			e, err := b.Consume(read, "svc@h", "", "", false, false)
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
	known(t, b, "owner@h")
	if _, err := b.Register(protocol.Record{Name: "  svc@h  ", Kind: "agent", Owner: "owner@h", Allow: []string{"peer@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "peer@h", Owner: "owner@h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: " peer@h", To: "svc@h ", Body: "hi"}); err != nil {
		t.Fatalf("send to the registered name failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, err := b.Consume(ctx, "svc@h", "", "", false, false)
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
	go b.Consume(ctx, "svc@h", "", "", false, false)
	waitForWaiters(t, b, "svc@h", 1)

	if _, err := b.Consume(ctx, "svc@h", "", "", false, false); err != ErrTwoReads {
		t.Fatalf("want ErrTwoReads, got %v", err)
	}
	done := make(chan error, 1)
	go func() { _, err := b.Consume(ctx, "svc@h", "t", "g", true, false); done <- err }()
	waitForWaiters(t, b, "svc@h", 2)
	cancel()
	if err := <-done; err == ErrTwoReads {
		t.Fatal("a filtered waiter must be allowed alongside the reader")
	}
}

// A pool says so, and then any number of them may block on one empty inbox.
// Each gets one message and no message goes to two — under -race, because
// the handoff is where a second reader would otherwise take a message twice.
// See docs/04-messaging.md#one-reader-per-inbox.
func TestSharingReadersEachTakeOneFromAnEmptyInbox(t *testing.T) {
	b := newBusWith(t, "pool@h", "sender@h")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	const workers = 4
	got := make(chan string, workers)
	errs := make(chan error, workers)
	for range workers {
		go func() {
			e, err := b.Consume(ctx, "pool@h", "", "", false, true)
			if err != nil {
				errs <- err
				return
			}
			got <- e.Body
		}()
	}
	waitForWaiters(t, b, "pool@h", workers)
	for i := range workers {
		if _, err := b.Send(protocol.Envelope{From: "sender@h", To: "pool@h", Body: fmt.Sprint(i)}); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	seen := map[string]bool{}
	for range workers {
		select {
		case err := <-errs:
			t.Fatalf("a sharing reader was refused: %v", err)
		case body := <-got:
			if seen[body] {
				t.Fatalf("%q went to two readers", body)
			}
			seen[body] = true
		case <-ctx.Done():
			t.Fatalf("only %d of %d workers were served", len(seen), workers)
		}
	}
}

// Sharing is both readers' word: one that wants the inbox to itself still
// gets the old refusal, whichever of the two arrived first.
func TestSharingIsRefusedBesideAReaderThatWantsTheInboxToItself(t *testing.T) {
	for _, first := range []bool{false, true} {
		b := newBusWith(t, "svc@h")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		go b.Consume(ctx, "svc@h", "", "", false, first)
		waitForWaiters(t, b, "svc@h", 1)
		if _, err := b.Consume(ctx, "svc@h", "", "", false, !first); err != ErrTwoReads {
			t.Fatalf("sharing=%v then %v: want ErrTwoReads, got %v", first, !first, err)
		}
		cancel()
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
	known(t, b, "a@srv")
	mustRegister(t, b, protocol.Record{Name: "jobs@srv", Allow: []string{"*"}, Kind: protocol.KindQueue, Owner: "a@srv"})
	known(t, b, "pub@srv")
	if _, err := b.Send(protocol.Envelope{From: "pub@srv", To: "jobs@srv", Topic: "jobs@srv", Body: "work"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, err := b.Consume(ctx, "jobs@srv", "", "", false, false)
	if err != nil || e.Body != "work" {
		t.Fatalf("reading the topic afterwards: %v %+v", err, e)
	}
}

// A copy each, into each subscriber's own inbox — and the topic keeps none
// of it. See docs/04-messaging.md#subscribers.
func TestPublishingToAPubSubTopicCopiesToEachSubscriber(t *testing.T) {
	b := New()
	known(t, b, "a@srv")
	mustRegister(t, b, protocol.Record{Name: "news@srv", Allow: []string{"*"}, Kind: protocol.KindPubSub, Owner: "a@srv"})
	known(t, b, "pub@srv")
	for _, s := range []string{"one@srv", "two@srv"} {
		known(t, b, s)
		if _, err := b.Subscribe(s, "news@srv", true); err != nil {
			t.Fatalf("subscribe %s: %v", s, err)
		}
	}
	if _, err := b.Send(protocol.Envelope{From: "pub@srv", To: "news@srv", Body: "x"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, s := range []string{"one@srv", "two@srv"} {
		e, err := b.Consume(ctx, s, "", "", false, false)
		if err != nil {
			t.Fatalf("%s got nothing: %v", s, err)
		}
		if e.Body != "x" || e.To != s {
			t.Fatalf("%s got %+v, want body x addressed to it", s, e)
		}
	}
	if r, ok := b.Lookup("a@srv", "news@srv"); !ok || r.Queued != 0 {
		t.Fatalf("the topic kept %d of its own", r.Queued)
	}
}

// A subscription is the topic's record, so restating the record must not
// throw it away — a service registers itself on every start.
func TestASubscriptionSurvivesTheTopicBeingRestated(t *testing.T) {
	b := New()
	known(t, b, "a@srv")
	mustRegister(t, b, protocol.Record{Name: "news@srv", Allow: []string{"*"}, Kind: protocol.KindPubSub, Owner: "a@srv"})
	known(t, b, "one@srv")
	if _, err := b.Subscribe("one@srv", "news@srv", true); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	mustRegister(t, b, protocol.Record{Name: "news@srv", Allow: []string{"*"}, Kind: protocol.KindPubSub, Owner: "a@srv", Descr: "again"})
	r, ok := b.Lookup("a@srv", "news@srv")
	if !ok || len(r.Subs) != 1 || r.Subs[0] != "one@srv" {
		t.Fatalf("subscribers after a restate: %v", r.Subs)
	}
	// And a caller cannot claim one by stating it.
	mustRegister(t, b, protocol.Record{Name: "news@srv", Allow: []string{"*"}, Kind: protocol.KindPubSub, Owner: "a@srv", Subs: []string{"intruder@srv"}})
	r, _ = b.Lookup("a@srv", "news@srv")
	if len(r.Subs) != 1 || r.Subs[0] != "one@srv" {
		t.Fatalf("a stated subscriber was taken: %v", r.Subs)
	}
	// Nor on a name nobody registered before, where there is no stored
	// record to restore over it.
	mustRegister(t, b, protocol.Record{Name: "fresh@srv", Kind: protocol.KindPubSub, Owner: "a@srv", Subs: []string{"intruder@srv"}})
	if r, _ := b.Lookup("a@srv", "fresh@srv"); len(r.Subs) != 0 {
		t.Fatalf("a stated subscriber was taken on a new topic: %v", r.Subs)
	}
}

// Every answer that carries liveness is built here, and it is the one place
// that could hand a caller's own claim back to them as though the daemon had
// observed it. Register cleans a stated record on the way in; this is the
// other end of the same rule, and without it the guard has no check.
func TestAnAnswerNeverCarriesLivenessItWasHandedIn(t *testing.T) {
	b := New()
	known(t, b, "claimer@srv")
	got := b.withLiveness("claimer@srv", protocol.Record{Kind: protocol.KindAgent, Name: "claimer@srv", Reading: true, Readers: ptr(9), Queued: 9, In: 9, Out: 9,
		Dropped: 9, Expired: 9, Oldest: "99h",
	})
	if got.Reading || got.Readers == nil || *got.Readers != 0 || got.Queued != 0 || got.In != 0 || got.Out != 0 ||
		got.Dropped != 0 || got.Expired != 0 || got.Oldest != "" {
		t.Fatalf("liveness came back from the caller, not from the inbox: %+v", got)
	}
}

// A receipt rides the same topic and tag as the message it is about, so the
// caller's filtered wait sees it. See docs/04-messaging.md#receipts.
func TestAReceiptReachesTheCallersFilteredWait(t *testing.T) {
	b := New()
	known(t, b, "caller@srv")
	if _, err := b.Send(protocol.Envelope{
		From: "caller@srv", To: "caller@srv", Topic: "call", Tag: "t1", Receipt: "ack", Re: "abc",
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	e, err := b.Consume(ctx, "caller@srv", "call", "t1", true, false)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if e.Receipt != "ack" || e.Re != "abc" {
		t.Fatalf("the receipt lost its meaning: %+v", e)
	}
}

func mustRegister(t *testing.T, b *Bus, r protocol.Record) {
	t.Helper()
	// A fixture that states no kind means a name on this bus, not an external
	// service: the daemon's own default is the external case.
	if r.Kind == "" {
		r.Kind = protocol.KindAgent
	}
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
			known(t, b, "sink@h", "s@h")
			const n = 100
			for i := 0; i < n; i++ {
				if _, err := b.Send(protocol.Envelope{
					To: "sink@h", From: "s@h", Tag: fmt.Sprint(i), Body: bigBody(i),
				}); err != nil {
					t.Fatal(err)
				}
			}
			for _, tag := range tc.order(n) {
				if _, err := b.Consume(context.Background(), "sink@h", "", fmt.Sprint(tag), true, false); err != nil {
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
	provision(t, b, protocol.Record{Kind: protocol.KindAgent, Name: "ringy@h", Allow: []string{"s@h"}, Full: protocol.OverflowRing})
	known(t, b, "s@h")
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
	known(t, b, "w@h", "s@h")
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := b.Consume(context.Background(), "w@h", "t", "g", true, false); err != nil {
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
	known(t, b, "svc@h")
	if _, err := b.Configure("svc@h", "svc@h", []byte("{ \"k\" : \"v\" }\n")); err != nil {
		t.Fatal(err)
	}
	// Read it back the one way there is: Configure answers without it.
	stored, err := b.Config("svc@h", "svc@h")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(stored); got != `{"k":"v"}` {
		t.Fatalf("stored %q", got)
	}
	spaced, _ := b.Lookup("svc@h", "svc@h")
	known(t, b, "other@h")
	compact, err := b.Configure("other@h", "other@h", []byte(`{"k":"v"}`))
	if err != nil {
		t.Fatal(err)
	}
	if a, c := spaced.Public().ConfigSHA, compact.Public().ConfigSHA; a != c {
		t.Fatalf("whitespace changed the digest:\n%s\n%s", a, c)
	}
}

// A digest a caller made up is worse than no digest: the whole point of it is
// that someone holding a service's setup can tell whether it still matches
// (docs/03-services-and-topics.md#why-a-digest-at-all).
func TestARegistrationCannotClaimAConfiguration(t *testing.T) {
	b := New()
	known(t, b, "parf@srv1")
	rec, err := b.Register(protocol.Record{
		Name: "plain@h", Kind: protocol.KindAgent, Owner: "parf@srv1",
		Config: []byte(`{"smuggled":true}`), ConfigSHA: "forged-by-the-caller",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.ConfigSHA != "" || rec.Config != nil {
		t.Fatalf("registration carried a configuration in: sha=%q config=%s", rec.ConfigSHA, rec.Config)
	}
	if pub := rec.Public(); pub.ConfigSHA != "" {
		t.Fatalf("an unconfigured service answers with a digest: %q", pub.ConfigSHA)
	}

	// And configuring it for real produces one that is actually derived.
	cfg, err := b.Configure("plain@h", "parf@srv1", []byte(`{"k":"v"}`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Public().ConfigSHA == "forged-by-the-caller" || cfg.Public().ConfigSHA == "" {
		t.Fatalf("digest is %q", cfg.Public().ConfigSHA)
	}
}

// Re-registering must not let a caller replace the digest of a configuration
// it cannot read.
func TestReRegisteringKeepsTheRealDigest(t *testing.T) {
	b := New()
	known(t, b, "svc@h")
	if _, err := b.Configure("svc@h", "svc@h", []byte(`{"k":"v"}`)); err != nil {
		t.Fatal(err)
	}
	real, known := b.Lookup("svc@h", "svc@h")
	if !known {
		t.Fatal("the configured service is not registered")
	}
	again, err := b.Register(protocol.Record{Name: "svc@h", Kind: protocol.KindAgent, Owner: "svc@h", ConfigSHA: "forged"})
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Public().ConfigSHA; got != real.Public().ConfigSHA {
		t.Fatalf("re-registration changed the digest to %q", got)
	}
}

// An inbox for any name that ever asked is unbounded growth from the outside:
// every distinct caller leaves one behind, and none of them can ever receive
// anything, because Send refuses an unknown receiver.
func TestConsumingFromAnUnregisteredNameIsRefused(t *testing.T) {
	b := New()
	known(t, b, "somebody@h")
	// Two callers, two refusals, no inbox either way. A name that is nobody
	// is refused for being nobody, before the question of whose inbox it is;
	// a principal asking after a name that does not exist is told so.
	if _, err := b.Consume(context.Background(), "ghost@nowhere", "", "", false, false); !errors.Is(err, ErrNoPrincipal) {
		t.Fatalf("err = %v, want ErrNoPrincipal", err)
	}
	if _, err := b.ConsumeAs(context.Background(), "somebody@h", "ghost@nowhere", "", "", false, false); !errors.Is(err, ErrUnknown) {
		t.Fatalf("err = %v, want ErrUnknown", err)
	}
	b.mu.Lock()
	_, made := b.inboxes["ghost@nowhere"]
	b.mu.Unlock()
	if made {
		t.Fatal("the refused read left an inbox behind")
	}
}

// Live state is what the daemon observes, so a caller stating it is claiming
// a reader it does not have — the same shape as a forged digest.
func TestARegistrationCannotClaimLiveState(t *testing.T) {
	b := New()
	known(t, b, "o@h")
	rec, err := b.Register(protocol.Record{Name: "probe@h", Kind: "agent", Owner: "o@h", Reading: true, Readers: ptr(99), Queued: 77})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Reading || rec.Readers != nil || rec.Queued != 0 {
		t.Fatalf("the registration answer carried live state: reading=%v readers=%v queued=%d", rec.Reading, rec.Readers, rec.Queued)
	}
	got, ok := b.Lookup("probe@h", "probe@h")
	if !ok {
		t.Fatal("not registered")
	}
	if got.Reading || got.Readers == nil || *got.Readers != 0 {
		t.Fatalf("stored or absent reader state survived into a lookup: reading=%v readers=%v", got.Reading, got.Readers)
	}
	for _, r := range b.List("probe@h", "") {
		if r.Name == "probe@h" && (r.Reading || r.Readers == nil || *r.Readers != 0) {
			t.Fatalf("stored or absent reader state survived into a listing: reading=%v readers=%v", r.Reading, r.Readers)
		}
	}
}

// A topic mode the daemon does not know reads as a queue, which is the one
// A kind the daemon does not know is refused at registration, naming the set,
// because a stored record nothing can describe is worse than a refused one.
func TestAKindTheDaemonDoesNotKnowIsRefused(t *testing.T) {
	b := New()
	known(t, b, "o@h")
	for _, bad := range []string{"garbage", "generic", "topic"} {
		if _, err := b.Register(protocol.Record{Name: "t@h", Kind: bad, Owner: "o@h"}); !errors.Is(err, ErrKind) {
			t.Fatalf("kind %q: err = %v, want ErrKind", bad, err)
		}
	}
	for _, ok := range protocol.Kinds {
		r := protocol.Record{Name: "t-" + ok + "@h", Kind: ok, Owner: "o@h"}
		if ok == protocol.KindService {
			r.Addr, r.Proto = "db.example", "mysql"
		}
		if _, err := b.Register(r); err != nil {
			t.Fatalf("kind %q refused: %v", ok, err)
		}
	}
	// Stating nothing is the external case, and the external case has to say
	// where it is.
	if _, err := b.Register(protocol.Record{Name: "bare@h", Owner: "o@h"}); !errors.Is(err, ErrKind) {
		t.Fatalf("a bare registration: err = %v, want ErrKind", err)
	}
	if _, err := b.Register(protocol.Record{Name: "bare@h", Owner: "o@h", Addr: "x", Proto: "https"}); err != nil {
		t.Fatalf("a bare registration with an address: %v", err)
	}
	if r, ok := b.Lookup("o@h", "bare@h"); !ok || r.Kind != protocol.KindService {
		t.Fatalf("a bare registration stored %q, want %s", r.Kind, protocol.KindService)
	}
}

// withLiveness is the second half of the rule that live state is observed,
// never stored: whatever a record arrives holding, what leaves carries what
// the daemon sees. Register already refuses to store either field, so the
// clearing here is unreachable through the API — which is exactly why it is
// tested directly. A mutation of it survived the whole acceptance suite.
func TestLivenessIsAssignedNotMerged(t *testing.T) {
	b := New()
	known(t, b, "svc@h")
	dirty := protocol.Record{Kind: protocol.KindAgent, Name: "svc@h", Reading: true, Readers: ptr(99), Queued: 77}
	look := func(name string, r protocol.Record) protocol.Record {
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.withLiveness(name, r)
	}

	if got := look("svc@h", dirty); got.Reading || got.Readers == nil || *got.Readers != 0 || got.Queued != 0 {
		t.Fatalf("an idle empty inbox answered reading=%v readers=%v queued=%d", got.Reading, got.Readers, got.Queued)
	}
	if got := look("no-inbox@h", dirty); got.Reading || got.Readers == nil || *got.Readers != 0 || got.Queued != 0 {
		t.Fatalf("a name with no inbox answered reading=%v readers=%v queued=%d", got.Reading, got.Readers, got.Queued)
	}

	// A waiting unfiltered reader is reported, and only while it waits.
	done := make(chan struct{})
	go func() { defer close(done); _, _ = b.Consume(context.Background(), "svc@h", "", "", false, false) }()
	for deadline := time.Now().Add(2 * time.Second); !look("svc@h", protocol.Record{}).Reading; {
		if time.Now().After(deadline) {
			t.Fatal("a waiting unfiltered reader was never reported")
		}
		time.Sleep(2 * time.Millisecond)
	}
	known(t, b, "x@h")
	if _, err := b.Send(protocol.Envelope{From: "x@h", To: "svc@h", Body: "go"}); err != nil {
		t.Fatal(err)
	}
	<-done
	if got := look("svc@h", dirty); got.Reading {
		t.Fatal("the reader is gone and is still reported")
	}

	// A backlog is counted, not merely cleared.
	for i := 0; i < 3; i++ {
		if _, err := b.Send(protocol.Envelope{From: "x@h", To: "svc@h", Body: "q"}); err != nil {
			t.Fatal(err)
		}
	}
	if got := look("svc@h", protocol.Record{}); got.Queued != 3 {
		t.Fatalf("three queued messages reported as %d", got.Queued)
	}
}

// A disabled subscriber is skipped, and the skip is its own loss: nothing is
// queued for it and the publisher is told nothing, so the only place the gap
// can show is the subscriber's own drop count. Access taken away is not the
// same fact and is not counted — that subscriber is not entitled to the copy.
func TestAPublicationASubscriberIsTooDisabledToTakeCountsAsItsDrop(t *testing.T) {
	b := New()
	known(t, b, "a@srv", "pub@srv")
	mustRegister(t, b, protocol.Record{Name: "news@srv", Allow: []string{"*"}, Kind: protocol.KindPubSub, Owner: "a@srv"})
	for _, s := range []string{"live@srv", "off@srv", "barred@srv"} {
		mustRegister(t, b, protocol.Record{Name: s, Allow: []string{"*"}, Kind: "agent", Owner: "a@srv"})
		if _, err := b.Subscribe(s, "news@srv", true); err != nil {
			t.Fatalf("subscribe %s: %v", s, err)
		}
	}
	if _, err := b.Manage("a@srv", Management{Name: "off@srv", Disabled: ptr(true)}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	// Barred by the topic rather than turned off: the topic stops allowing it.
	if _, err := b.Manage("a@srv", Management{Name: "news@srv", Allow: ptr([]string{"live@srv", "off@srv", "pub@srv"})}); err != nil {
		t.Fatalf("narrow the topic: %v", err)
	}
	if _, err := b.Send(protocol.Envelope{From: "pub@srv", To: "news@srv", Body: "x"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	for _, c := range []struct {
		name    string
		queued  int
		dropped int
	}{
		{"live@srv", 1, 0},
		{"off@srv", 0, 1},
		{"barred@srv", 0, 0},
	} {
		r, ok := b.Lookup("a@srv", c.name)
		if !ok {
			t.Fatalf("%s is gone", c.name)
		}
		if r.Queued != c.queued || r.Dropped != c.dropped {
			t.Fatalf("%s holds %d and dropped %d, want %d and %d", c.name, r.Queued, r.Dropped, c.queued, c.dropped)
		}
	}
}

// A record has one shape, and every path that stores one asks the same
// question about it. Registration refusing a malformed record is not the same
// protection as a settings edit or a restore refusing it: those were separate
// code paths, and each one that skipped the check could put back a record the
// daemon could not serve.
func TestEveryPathThatStoresARecordAsksTheSameShapeQuestion(t *testing.T) {
	b := New()
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h")
	if _, err := b.Register(protocol.Record{
		Name: "db@h", Owner: "alice@h", Kind: protocol.KindService, Addr: "db.example:5432", Proto: "postgresql",
	}); err != nil {
		t.Fatal(err)
	}
	// Manage names one field at a time, so the record it would leave behind is
	// what has to be legal, not the field the caller happened to mention.
	for _, change := range []Management{
		{Name: "db@h", Addr: ptr("")},
		{Name: "db@h", Proto: ptr("")},
	} {
		if _, err := b.Manage("alice@h", change); !errors.Is(err, ErrKind) {
			t.Errorf("a settings edit emptied a service endpoint: %+v, %v", change, err)
		}
	}
	// Falsifiable: the same edit through the same path is accepted when what
	// it leaves behind is still a reachable service.
	if got, err := b.Manage("alice@h", Management{Name: "db@h", Addr: ptr("db.internal:5432")}); err != nil || got.Addr != "db.internal:5432" {
		t.Fatalf("a real endpoint change was refused: %+v, %v", got, err)
	}

	// Restore is the third path. A snapshot is not a caller and gets no
	// weaker question: EstablishDaemonOwner is where a refused restore stops
	// the daemon rather than letting it serve state it cannot describe.
	for name, record := range map[string]protocol.Record{
		"unknown kind":        {Name: "x@h", Owner: "alice@h", Kind: "generic"},
		"service, no address": {Name: "x@h", Owner: "alice@h", Kind: protocol.KindService, Proto: "https"},
		"service, no proto":   {Name: "x@h", Owner: "alice@h", Kind: protocol.KindService, Addr: "h:1"},
		"personal queue":      {Name: "x@h", Owner: "alice@h", Kind: protocol.KindQueue, Personal: true},
	} {
		restarted := New()
		restarted.Restore(ports.Snapshot{
			Users:   []protocol.User{{Name: "alice@h", State: "active"}},
			Records: []protocol.Record{record},
		})
		if err := restarted.EstablishDaemonOwner("alice@h"); err == nil {
			t.Errorf("a snapshot holding a %s record started a daemon", name)
		}
	}
	// Falsifiable: a snapshot of records this version can serve restores.
	restarted := New()
	restarted.Restore(ports.Snapshot{
		Users: []protocol.User{{Name: "alice@h", State: "active"}},
		Records: []protocol.Record{
			// The Personal record is listed BEFORE the agent its ACL names, so
			// this also fails if Personal is validated mid-restore.
			{Name: "mine@h", Owner: "alice@h", Kind: protocol.KindAgent, Personal: true, Allow: []string{"peer@h"}},
			{Name: "peer@h", Owner: "alice@h", Kind: protocol.KindAgent},
			{Name: "db@h", Owner: "alice@h", Kind: protocol.KindService, Addr: "h:1", Proto: "https"},
		},
	})
	if err := restarted.EstablishDaemonOwner("alice@h"); err != nil {
		t.Fatalf("a restorable snapshot was refused: %v", err)
	}
}

// An enrolled person is a person: the realm proved a key for a human login and
// the daemon makes them a user, so their own queue is a user's, not an agent's.
func TestEnrolmentRecordsAPersonAsOne(t *testing.T) {
	b := New()
	b.Directories(map[string]ports.Directory{"github": &profileDirectory{
		entry: ports.DirectoryEntry{Keys: []string{"key-one"}, Profile: ports.DirectoryProfile{Login: "alice"}},
	}}, profileSignatures{})
	nonce, err := b.Challenge("alice@github")
	if err != nil {
		t.Fatal(err)
	}
	record, err := b.Enrol(nonce, "good")
	if err != nil {
		t.Fatal(err)
	}
	if record.Kind != protocol.KindUser {
		t.Errorf("enrolment registered kind %q, want %s", record.Kind, protocol.KindUser)
	}
	// Falsifiable the other way: the kind follows from being a person, not
	// from the enrolment path, so an ordinary agent registered beside it stays
	// an agent.
	b.SetDaemonOwner("admin@h")
	known(t, b, "owner@h")
	if _, err := b.Register(protocol.Record{Name: "bot@h", Owner: "owner@h", Kind: protocol.KindAgent}); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.Lookup("owner@h", "bot@h"); got.Kind != protocol.KindAgent {
		t.Errorf("an ordinary agent registered as kind %q", got.Kind)
	}
}
