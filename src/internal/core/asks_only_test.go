package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

func asksRead(b *Bus, caller, name string, wait time.Duration) (protocol.Envelope, error) {
	ctx, cancel := context.WithTimeout(WithAsksOnly(context.Background()), wait)
	defer cancel()
	return b.ConsumeAs(ctx, caller, name, "", "", false, false)
}

func sendAs(t *testing.T, b *Bus, body, ask string) {
	t.Helper()
	if _, err := b.Send(protocol.Envelope{From: "#peer@h", To: "#svc@h", Body: body, Ask: ask}); err != nil {
		t.Fatal(err)
	}
}

// An asks-only read takes the oldest queued message with an ask, leaves every
// other message in its place and order, and counts only what it took.
func TestAsksOnlyReadTakesAsksAndLeavesTheQueueAlone(t *testing.T) {
	b := newBusWith(t, "#svc@h", "#peer@h")
	sendAs(t, b, "work one", "")
	sendAs(t, b, "", "help")
	sendAs(t, b, "work two", "")
	sendAs(t, b, "", "status")
	for _, want := range []string{"help", "status"} {
		e, err := asksRead(b, "#svc@h", "#svc@h", time.Second)
		if err != nil || e.Ask != want {
			t.Fatalf("asks-only read: %+v %v, want the %s ask", e, err, want)
		}
	}
	if _, err := asksRead(b, "#svc@h", "#svc@h", 30*time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("with only work queued an asks-only read got %v, want it to wait out", err)
	}
	r, _ := b.Lookup("#svc@h", "#svc@h")
	if r.Queued != 2 || r.Out != 2 {
		t.Fatalf("queued %d out %d, want the two work messages queued and two taken", r.Queued, r.Out)
	}
	for _, want := range []string{"work one", "work two"} {
		if e := readOne(t, b, "#svc@h"); e.Body != want {
			t.Fatalf("the ordinary reader got %q, want %q in order", e.Body, want)
		}
	}
}

// A waiting asks-only read is handed an ask that arrives, and never an ordinary
// message, which stays queued.
func TestWaitingAsksOnlyReadIsHandedAsksOnly(t *testing.T) {
	b := newBusWith(t, "#svc@h", "#peer@h")
	got := make(chan protocol.Envelope, 1)
	go func() {
		if e, err := asksRead(b, "#svc@h", "#svc@h", 2*time.Second); err == nil {
			got <- e
		}
	}()
	waitForWaiters(t, b, "#svc@h", 1)
	sendAs(t, b, "work", "")
	select {
	case e := <-got:
		t.Fatalf("the asks-only read was handed ordinary work %q", e.Body)
	case <-time.After(50 * time.Millisecond):
	}
	sendAs(t, b, "", "help")
	select {
	case e := <-got:
		if e.Ask != "help" {
			t.Fatalf("handed %+v, want the help ask", e)
		}
	case <-time.After(time.Second):
		t.Fatal("the asks-only read never got the ask")
	}
	if r, _ := b.Lookup("#svc@h", "#svc@h"); r.Queued != 1 {
		t.Fatalf("queued %d, want the work message still queued", r.Queued)
	}
}

// An asks-only read and the unfiltered reader wait together: neither is refused
// as a second reader, asks go to the asks-only read and work to the other.
func TestAsksOnlyReadCoexistsWithTheUnfilteredReader(t *testing.T) {
	b := newBusWith(t, "#svc@h", "#peer@h")
	work, asks := make(chan protocol.Envelope, 1), make(chan protocol.Envelope, 1)
	errs := make(chan error, 2)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		e, err := b.Consume(ctx, "#svc@h", "", "", false, false)
		errs <- err
		if err == nil {
			work <- e
		}
	}()
	waitForWaiters(t, b, "#svc@h", 1)
	go func() {
		e, err := asksRead(b, "#svc@h", "#svc@h", 3*time.Second)
		errs <- err
		if err == nil {
			asks <- e
		}
	}()
	waitForWaiters(t, b, "#svc@h", 2)
	select {
	case err := <-errs:
		t.Fatalf("one of the two readers was refused or ended: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	sendAs(t, b, "", "help")
	sendAs(t, b, "work", "")
	if e := <-asks; e.Ask != "help" {
		t.Fatalf("the asks-only read got %+v, want the help ask", e)
	}
	if e := <-work; e.Body != "work" || e.Ask != "" {
		t.Fatalf("the unfiltered reader got %+v, want the work", e)
	}
}

// An asks-only read needs the consume authority the caller already has: a
// caller the ACL refuses is refused, and nothing is taken.
func TestAsksOnlyReadKeepsTheConsumeAuthority(t *testing.T) {
	b := New()
	known(t, b, "owner@h", "intruder@h", "#peer@h")
	if _, err := b.Register(protocol.Record{Name: "#svc@h", Kind: protocol.KindAgent, Owner: "owner@h", Allow: []string{"owner@h", "#peer@h"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Send(protocol.Envelope{From: "#peer@h", To: "#svc@h", Ask: "help"}); err != nil {
		t.Fatal(err)
	}
	if _, err := asksRead(b, "intruder@h", "#svc@h", 30*time.Millisecond); !errors.Is(err, ErrNotAllow) {
		t.Fatalf("the intruder's asks-only read: %v, want it refused", err)
	}
	if r, _ := b.Lookup("owner@h", "#svc@h"); r.Queued != 1 {
		t.Fatalf("the refused read took the ask: queued %d", r.Queued)
	}
	if e, err := asksRead(b, "owner@h", "#svc@h", time.Second); err != nil || e.Ask != "help" {
		t.Fatalf("the owner's asks-only read: %+v %v", e, err)
	}
}

func TestAsksValueIsCheckedByName(t *testing.T) {
	if err := CheckAsks("only"); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"", "skip", "ONLY", "none", "1"} {
		if err := CheckAsks(v); !errors.Is(err, ErrBadName) || !strings.Contains(err.Error(), "asks") {
			t.Errorf("%q: %v, want a refusal naming asks", v, err)
		}
	}
}
