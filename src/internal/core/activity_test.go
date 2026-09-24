package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/activity"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

// testClock is the time a test sets; the bus reads it through Clock.
type testClock struct{ t time.Time }

func (c *testClock) now() time.Time { return c.t }

func localAt(h, m int) time.Time { return time.Date(2026, time.September, 23, h, m, 0, 0, time.Local) }

func last(t *testing.T, b *Bus, caller, name string) (closed, open ActivityPoint) {
	t.Helper()
	day, err := b.Activity(caller, name)
	if err != nil {
		t.Fatal(err)
	}
	if len(day) != activity.Slots {
		t.Fatalf("%d slots, want a day of %d", len(day), activity.Slots)
	}
	return day[len(day)-2], day[len(day)-1]
}

func TestActivityIsFilteredAndSummed(t *testing.T) {
	clock := &testClock{localAt(10, 0)}
	b := New()
	b.Clock(clock.now)
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h")
	b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#visible@h", Owner: "alice@h", Allow: []string{"alice@h"}, Bound: 1, Full: "ring"})
	b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#hidden@h", Owner: "bob@h", Allow: []string{"bob@h"}})
	b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#asleep@h", Owner: "alice@h", Allow: []string{"alice@h"}})
	b.Send(protocol.Envelope{From: "alice@h", To: "#visible@h", Body: "first"})
	b.Send(protocol.Envelope{From: "alice@h", To: "#visible@h", Body: "second"})
	b.Send(protocol.Envelope{From: "bob@h", To: "#hidden@h", Body: "secret"})
	b.Send(protocol.Envelope{From: "alice@h", To: "#asleep@h", Body: "asleep"})
	b.Consume(context.Background(), "#visible@h", "", "", false, false)
	b.Send(protocol.Envelope{From: "alice@h", To: "#visible@h", Body: "expires", TTL: "1ns"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b.Consume(ctx, "#visible@h", "", "", false, false)
	b.RecordRefusal("#VISIBLE@h")
	b.Refuse("acl")
	b.Refuse("auth")
	inactive := protocol.StatusInactive
	if _, err := b.Manage("alice@h", Management{Name: "#asleep@h", Status: &inactive}); err != nil {
		t.Fatal(err)
	}
	clock.t = localAt(10, 10)
	b.TickActivity(clock.t)
	closed, open := last(t, b, "alice@h", "#visible@h")
	if want := (Counts{In: 3, Out: 1, Dropped: 1, Expired: 1, Refused: 1}); closed.Counts != want || !closed.At.Equal(localAt(10, 0)) {
		t.Fatalf("10:00 of #visible@h: %s %+v, want %+v", closed.At, closed.Counts, want)
	}
	if open.Counts != (Counts{}) || !open.At.Equal(localAt(10, 10)) {
		t.Fatalf("open slot: %s %+v", open.At, open.Counts)
	}
	b.Send(protocol.Envelope{From: "alice@h", To: "#visible@h", Body: "live"})
	if _, open = last(t, b, "alice@h", "#visible@h"); open.In != 1 {
		t.Fatalf("the open slot is read live: %+v", open.Counts)
	}
	if closed, _ := last(t, b, "alice@h", ""); closed.In != 3 {
		t.Fatalf("all visible to alice: %+v; a hidden or inactive record leaked in", closed.Counts)
	}
	if closed, _ := last(t, b, "admin@h", ""); closed.In != 4 || closed.Refused != 2 {
		t.Fatalf("all visible to the daemon Owner: %+v, want every live record and the two node refusals", closed.Counts)
	}
	if _, err := b.Activity("alice@h", "#hidden@h"); !errors.Is(err, ErrUnknown) {
		t.Fatal("forbidden record's day returned")
	}
	if _, err := b.Activity("alice@h", "#asleep@h"); !errors.Is(err, ErrUnknown) {
		t.Fatal("an inactive record's day was served")
	}
}

// A restart keeps the day, the node's refusals included; the time the daemon
// was down reads zero; a refusal alone is reason to save.
func TestActivitySurvivesARestart(t *testing.T) {
	clock := &testClock{localAt(11, 50)}
	st := memory.NewState()
	b := New()
	b.Clock(clock.now)
	b.Persistence(st)
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h")
	b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "alice@h", Allow: []string{"*"}})
	b.TickActivity(clock.t)
	b.Send(protocol.Envelope{From: "alice@h", To: "#svc@h", Body: "one"})
	b.Send(protocol.Envelope{From: "alice@h", To: "#svc@h", Body: "two"})
	b.Refuse("auth")
	clock.t = localAt(11, 55)
	if err := b.FlushQueues(false); err != nil {
		t.Fatal(err)
	}
	clock.t = localAt(12, 0)
	b.TickActivity(clock.t)
	b.RecordRefusal("#svc@h")
	b.Refuse("auth")
	clock.t = localAt(12, 5) // the save is after 12:00; the tick put them there
	// The graceful stop: the days with the open slot so far, then the queues.
	if err := b.FlushActivity(); err != nil {
		t.Fatal(err)
	}
	if err := b.FlushQueues(true); err != nil {
		t.Fatal(err)
	}
	saved := stored(t, b, st)
	clock.t = localAt(13, 0).Add(10 * time.Second)
	back := New()
	back.Clock(clock.now)
	back.Restore(saved)
	back.Persistence(st)
	back.RestoreActivityDays()
	day, err := back.Activity("alice@h", "#svc@h")
	if err != nil {
		t.Fatal(err)
	}
	n := len(day)
	if at := day[n-8]; !at.At.Equal(localAt(11, 50)) || at.In != 2 {
		t.Fatalf("11:50 after restart: %s %+v, want the two sent", at.At, at.Counts)
	}
	if at := day[n-7]; !at.At.Equal(localAt(12, 0)) || at.Refused != 1 {
		t.Fatalf("12:00 after restart: %+v, want the refusal saved on its own", at.Counts)
	}
	for _, s := range day[n-6:] {
		if s.Counts != (Counts{}) {
			t.Fatalf("down slot %s holds %+v", s.At, s.Counts)
		}
	}
	all, _ := back.Activity("admin@h", "")
	if all[n-8].Refused != 1 || all[n-7].Refused != 1 {
		t.Fatalf("node refusals at 11:50 and 12:00 after restart: %d and %d, want 1 and 1", all[n-8].Refused, all[n-7].Refused)
	}
}

// A read between a boundary and the next minute tick already has the new
// slot open: what arrives after it is counted there, not in the slot before.
func TestActivityReadsAtTheBusClock(t *testing.T) {
	clock := &testClock{localAt(10, 0)}
	b := New()
	b.Clock(clock.now)
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h")
	b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "alice@h", Allow: []string{"*"}})
	b.Send(protocol.Envelope{From: "alice@h", To: "#svc@h", Body: "at 10:00"})
	b.Refuse("auth")
	clock.t = localAt(10, 10).Add(20 * time.Second) // no minute tick yet
	last(t, b, "alice@h", "#svc@h")
	last(t, b, "admin@h", "")
	b.Send(protocol.Envelope{From: "alice@h", To: "#svc@h", Body: "at 10:10"})
	b.Refuse("auth")
	if closed, open := last(t, b, "alice@h", "#svc@h"); closed.In != 1 || open.In != 1 {
		t.Fatalf("read before the tick: 10:00=%d 10:10=%d, want 1 and 1", closed.In, open.In)
	}
	if closed, open := last(t, b, "admin@h", ""); closed.Refused != 1 || open.Refused != 1 {
		t.Fatalf("node refusals read before the tick: 10:00=%d 10:10=%d, want 1 and 1", closed.Refused, open.Refused)
	}
	fresh := New()
	fresh.Clock(clock.now)
	fresh.SetDaemonOwner("admin@h")
	fresh.Refuse("auth")
	fresh.Refuse("auth")
	if _, open := last(t, fresh, "admin@h", ""); open.Refused != 2 {
		t.Fatalf("a node ring never ticked reads %d refusals, want both", open.Refused)
	}
}

// A damaged stored day is reported once, starts empty, and is saved again so
// the next start reads it.
func TestADamagedStoredDayIsReportedOnce(t *testing.T) {
	clock := &testClock{localAt(9, 0)}
	st := memory.NewState()
	b := New()
	b.Clock(clock.now)
	b.Persistence(st)
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h")
	b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#svc@h", Owner: "alice@h", Allow: []string{"*"}})
	b.Send(protocol.Envelope{From: "alice@h", To: "#svc@h", Body: "x"})
	if err := st.SaveQueues([]ports.Queue{{Name: "#svc@h", In: 1, Activity: []byte{1, 3, 5}}}, nil, true); err != nil {
		t.Fatal(err)
	}
	saved := stored(t, b, st)
	restart := func(s ports.Snapshot) (*Bus, *reports) {
		rep := &reports{}
		r := New()
		r.Clock(clock.now)
		r.Journal(rep)
		r.Persistence(st)
		r.Restore(s)
		return r, rep
	}
	back, rep := restart(saved)
	var warned int
	for _, l := range rep.lines {
		if strings.HasPrefix(l, ports.Warning.String()+": stored activity of #svc@h is unreadable") {
			warned++
		}
	}
	if warned != 1 {
		t.Fatalf("%d warnings for the damaged day, want 1: %v", warned, rep.lines)
	}
	if _, open := last(t, back, "alice@h", "#svc@h"); open.In != 0 {
		t.Fatalf("the damaged day did not start empty: %+v", open.Counts)
	}
	if err := back.FlushQueues(true); err != nil {
		t.Fatal(err)
	}
	if _, rep = restart(stored(t, back, st)); len(rep.lines) != 0 {
		t.Fatalf("the next start still reports: %v", rep.lines)
	}
}

// Removal takes the day with it in its own commit; a transfer keeps it.
func TestActivityGoesWithRemovalAndStaysWithTransfer(t *testing.T) {
	clock := &testClock{localAt(9, 0)}
	st := memory.NewState()
	b := New()
	b.Clock(clock.now)
	b.Persistence(st)
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h")
	for _, name := range []string{"#kept@h", "#gone@h"} {
		b.Register(protocol.Record{Kind: protocol.KindAgent, Name: name, Owner: "alice@h", Allow: []string{"*"}})
		b.Send(protocol.Envelope{From: "alice@h", To: name, Body: "x"})
	}
	clock.t = localAt(9, 10)
	b.TickActivity(clock.t)
	if err := b.FlushQueues(false); err != nil {
		t.Fatal(err)
	}
	bob := "bob@h"
	if _, err := b.Manage("alice@h", Management{Name: "#kept@h", Owner: &bob}); err != nil {
		t.Fatal(err)
	}
	b.Consume(context.Background(), "#gone@h", "", "", false, false)
	commits := st.Commits
	if err := b.Unregister("#gone@h", "alice@h"); err != nil {
		t.Fatal(err)
	}
	if st.Commits != commits+1 {
		t.Fatalf("removal took %d commits", st.Commits-commits)
	}
	saved := stored(t, b, st)
	for _, q := range saved.Queues {
		if q.Name == "#gone@h" {
			t.Fatalf("the removed record's queue is still stored: %d bytes", len(q.Activity))
		}
	}
	days, _ := st.ActivityDays(0, 999999)
	for _, d := range days {
		if d.Name == "#gone@h" {
			t.Fatalf("the removed record's day %d is still stored", d.Date)
		}
	}
	back := New()
	back.Clock(clock.now)
	back.Restore(saved)
	back.Persistence(st)
	back.RestoreActivityDays()
	if closed, _ := last(t, back, "bob@h", "#kept@h"); closed.In != 1 {
		t.Fatalf("the transferred record lost its day: %+v", closed.Counts)
	}
	back.SetDaemonOwner("admin@h")
	back.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#gone@h", Owner: "alice@h", Allow: []string{"*"}})
	if closed, _ := last(t, back, "alice@h", "#gone@h"); closed.In != 0 {
		t.Fatalf("a new record under a removed name inherited its day: %+v", closed.Counts)
	}
}

// stored is what a restart reads: the queues and activity as the store holds
// them, with the users and records a fixture put in memory only.
func stored(t *testing.T, b *Bus, st *memory.State) ports.Snapshot {
	t.Helper()
	saved, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	mem := b.Snapshot()
	saved.Users, saved.Records = mem.Users, mem.Records
	return saved
}
