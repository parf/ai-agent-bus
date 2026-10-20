package core

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/protocol"
)

// A lasting state met by every message — here a paused subscriber — writes
// one warning a minute, not one per message, and the next line says how many
// it held back; the drop is still counted on every copy
// (issues/done/2026-10/2026-10-17.md#A8).
func TestALastingWarningIsWrittenOnceAMinute(t *testing.T) {
	b := New()
	rep := &reports{}
	b.Journal(rep)
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	b.clock = func() time.Time { return now }
	known(t, b, "a@srv", "pub@srv")
	mustRegister(t, b, protocol.Record{Name: "news@srv", Allow: []string{"pub@srv"}, Kind: protocol.KindPubSub, Owner: "a@srv"})
	for _, s := range []string{"#live@srv", "#off@srv"} {
		mustRegister(t, b, protocol.Record{Name: s, Allow: []string{"*"}, Kind: "agent", Owner: "a@srv"})
	}
	delivers(t, b, "a@srv", "news@srv", "#live@srv", "#off@srv")
	if _, err := b.Manage("a@srv", Management{Name: "#off@srv", Status: ptr(protocol.StatusInactive)}); err != nil {
		t.Fatal(err)
	}
	about := func() (n int, last string) {
		rep.mu.Lock()
		defer rep.mu.Unlock()
		for _, l := range rep.lines {
			if strings.Contains(l, "#off@srv") {
				n, last = n+1, l
			}
		}
		return n, last
	}
	publish := func(k int) {
		for i := 0; i < k; i++ {
			if _, err := b.Send(protocol.Envelope{From: "pub@srv", To: "news@srv", Body: "x"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	publish(100)
	if n, _ := about(); n != 1 {
		t.Fatalf("100 publications past a paused subscriber wrote %d warnings, want 1", n)
	}
	now = now.Add(repeatEvery - time.Second)
	publish(1)
	if n, _ := about(); n != 1 {
		t.Fatalf("within the minute: %d warnings, want 1", n)
	}
	now = now.Add(time.Second)
	publish(1)
	if n, last := about(); n != 2 || !strings.Contains(last, "and 100 more like it") {
		t.Fatalf("after the minute: %d warnings, the last %q; want 2, saying 100 more", n, last)
	}
	publish(1)
	if n, _ := about(); n != 2 {
		t.Fatalf("the minute after the summary: %d warnings, want 2", n)
	}
	if in := b.inboxes["#off@srv"]; in == nil || in.dropped != 103 {
		t.Fatalf("the paused subscriber's drops are not all counted: %+v", in)
	}
}

func TestRepeatedWarningsIgnoreCallerChosenCountsAndStayBounded(t *testing.T) {
	t.Run("counts", func(t *testing.T) {
		b := New()
		rep := &reports{}
		b.Journal(rep)
		for i := 0; i < 1100; i++ {
			b.warnRepeated("caller stated %d roles", i)
		}
		if len(b.repeated) != 1 || len(rep.lines) != 1 {
			t.Fatalf("caller counts produced %d warning keys and %d lines", len(b.repeated), len(rep.lines))
		}
	})
	t.Run("capacity", func(t *testing.T) {
		b := New()
		b.repeated = map[string]*repeat{}
		for i := 0; i < 1024; i++ {
			b.repeated[fmt.Sprint(i)] = &repeat{at: b.clock()}
		}
		b.warnRepeated("another warning")
		if len(b.repeated) > 1024 {
			t.Fatalf("warning map has %d entries, exceeds 1024", len(b.repeated))
		}
	})
}
