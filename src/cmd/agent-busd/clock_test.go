package main

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/callstats"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// fakeClock drives everyMinute: each wait it is asked for moves the clock on
// by exactly that much, and the loop stops after the given number of wakes.
type fakeClock struct {
	now   time.Time
	waits []time.Duration
	wakes int
	done  chan struct{}
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.waits = append(c.waits, d)
	ch := make(chan time.Time, 1)
	if c.wakes == 0 {
		close(c.done)
		return ch
	}
	c.wakes--
	c.now = c.now.Add(d)
	ch <- c.now
	return ch
}

// A daemon started at 10:07:30 reads its call counter at 10:08:00 and every
// minute of the clock after, so an hour of the loop leaves 61 readings a
// minute apart and "Calls, minute" is one minute.
func TestCallCounterIsReadEveryMinuteOfTheClock(t *testing.T) {
	var calls atomic.Uint64
	history := callstats.New(&calls)
	start := time.Date(2026, time.September, 23, 10, 7, 30, 0, time.Local)
	clock := &fakeClock{now: start, wakes: 61, done: make(chan struct{})}
	history.Sample(start)
	tick := onMinute(history, core.New())
	everyMinute(clock.done, clock.Now, clock.After, func(at time.Time) {
		calls.Add(1)
		tick(at)
	})
	if clock.waits[0] != 30*time.Second || clock.waits[1] != time.Minute || clock.waits[60] != time.Minute {
		t.Fatalf("waits %v: the first reading must fall at 10:08:00, the rest a minute apart", clock.waits[:3])
	}
	if got := clock.now; got.Second() != 0 || got.Minute() != 8 || got.Hour() != 11 {
		t.Fatalf("clock ended at %s, want 11:08:00", got)
	}
	windows := history.Snapshot(clock.now).Windows
	if m := windows[0]; !m.Available || m.Observed != "1m0s" || m.Count != 1 {
		t.Fatalf("Calls, minute: %+v, want one call over one minute", m)
	}
	if h := windows[1]; !h.Available || h.Observed != "1h0m0s" || h.Count != 60 {
		t.Fatalf("Calls, hour: %+v, want 60 calls over one hour", h)
	}
}

// The same minute tick closes the bus's activity slots on the clock: a
// daemon started at 10:07:30 closes its first slot at 10:10, and what
// arrives after that is the 10:10 slot's.
func TestActivitySlotsCloseOnTheClock(t *testing.T) {
	start := time.Date(2026, time.September, 23, 10, 7, 30, 0, time.Local)
	clock := &fakeClock{now: start, wakes: 3, done: make(chan struct{})}
	b := core.New()
	b.Clock(clock.Now)
	b.SetDaemonOwner("owner@h")
	if _, err := b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "owner@h"}); err != nil {
		t.Fatal(err)
	}
	send := func() {
		if _, err := b.Send(protocol.Envelope{From: "owner@h", To: "#svc@h", Body: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	tick := onMinute(callstats.New(new(atomic.Uint64)), b)
	everyMinute(clock.done, clock.Now, clock.After, func(at time.Time) {
		tick(at)
		send()
	})
	clock.now = clock.now.Add(30 * time.Second)
	day, err := b.Activity("owner@h", "#svc@h")
	if err != nil {
		t.Fatal(err)
	}
	closed, open := day[len(day)-2], day[len(day)-1]
	if closed.At.Hour() != 10 || closed.At.Minute() != 0 || closed.In != 2 || open.At.Minute() != 10 || open.In != 1 {
		t.Fatalf("10:00 %s=%d and 10:10 %s=%d; want 2 and 1", closed.At, closed.In, open.At, open.In)
	}
}
