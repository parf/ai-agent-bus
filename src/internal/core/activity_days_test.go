package core

import (
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/activity"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/memory"
)

func dayAt(day, h, m int) time.Time {
	return time.Date(2026, time.September, day, h, m, 0, 0, time.Local)
}

func daysBus(t *testing.T, clock *testClock) (*Bus, *memory.State) {
	t.Helper()
	st := memory.NewState()
	b := New()
	b.Clock(clock.now)
	b.Persistence(st)
	b.SetDaemonOwner("admin@h")
	known(t, b, "alice@h", "bob@h")
	b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#busy@h", Owner: "alice@h", Allow: []string{"*"}})
	b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#quiet@h", Owner: "alice@h", Allow: []string{"*"}})
	b.Register(protocol.Record{Kind: protocol.KindAgent, Name: "#private@h", Owner: "alice@h"})
	return b, st
}

func rows(t *testing.T, st *memory.State) map[string][]int {
	t.Helper()
	all, err := st.ActivityDays(0, 999999)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]int{}
	for _, d := range all {
		out[d.Name] = append(out[d.Name], d.Date)
	}
	return out
}

// A boundary writes the day of every slot that counted something, and nothing
// for a name that stayed silent; between boundaries it writes nothing at all.
func TestABoundaryWritesOnlyTheDaysThatMoved(t *testing.T) {
	clock := &testClock{dayAt(23, 23, 40)}
	b, st := daysBus(t, clock)
	b.TickActivity(clock.t)
	b.Send(protocol.Envelope{From: "alice@h", To: "#busy@h", Body: "x"})
	clock.t = dayAt(23, 23, 45)
	b.TickActivity(clock.t)
	if r := rows(t, st); len(r) != 0 {
		t.Fatalf("inside a slot the tick wrote %v", r)
	}
	clock.t = dayAt(23, 23, 50)
	b.TickActivity(clock.t)
	r := rows(t, st)
	if len(r["#busy@h"]) != 1 || r["#busy@h"][0] != 260923 {
		t.Fatalf("after 23:50: %v", r)
	}
	if _, wrote := r["#quiet@h"]; wrote {
		t.Fatal("a silent record got a row")
	}
	// The slot that closes at midnight belongs to the day before.
	b.Send(protocol.Envelope{From: "alice@h", To: "#busy@h", Body: "y"})
	clock.t = dayAt(24, 0, 0)
	b.TickActivity(clock.t)
	if r := rows(t, st); len(r["#busy@h"]) != 1 {
		t.Fatalf("23:50 closing at midnight wrote %v", r)
	}
	days, _ := b.ActivityDays("alice@h", "#busy@h", 260923, 260923)
	if got := days[0].Slots[142].In + days[0].Slots[143].In; got != 2 {
		t.Fatalf("23 Sep holds %d, want 2", got)
	}
}

// A range reads older days from the store and the ring's own days live; the
// sum covers only what the caller may see.
func TestARangeReadsStoredDaysAndTheLiveRing(t *testing.T) {
	clock := &testClock{dayAt(20, 10, 0)}
	b, _ := daysBus(t, clock)
	b.TickActivity(clock.t)
	for _, to := range []string{"#busy@h", "#private@h"} {
		b.Send(protocol.Envelope{From: "alice@h", To: to, Body: "old"})
	}
	clock.t = dayAt(20, 10, 10)
	b.TickActivity(clock.t)
	// Four days on, the 20th is only in the store.
	clock.t = dayAt(24, 9, 0)
	b.TickActivity(clock.t)
	b.Send(protocol.Envelope{From: "alice@h", To: "#busy@h", Body: "now"})
	days, err := b.ActivityDays("alice@h", "", 260920, 260924)
	if err != nil || len(days) != 5 {
		t.Fatalf("five days: %d %v", len(days), err)
	}
	in := func(d ActivityDay) (n int) {
		for _, s := range d.Slots {
			n += s.In
		}
		return
	}
	if in(days[0]) != 2 || in(days[4]) != 1 || in(days[2]) != 0 {
		t.Fatalf("20th %d, 22nd %d, 24th %d; want 2, 0, 1", in(days[0]), in(days[2]), in(days[4]))
	}
	if !days[0].Slots[60].At.Equal(dayAt(20, 10, 0)) {
		t.Fatalf("slot 60 of the 20th starts %s", days[0].Slots[60].At)
	}
	// bob sees #busy only: the private record's traffic is not in his sum.
	bobs, _ := b.ActivityDays("bob@h", "", 260920, 260920)
	if in(bobs[0]) != 1 {
		t.Fatalf("bob's 20th sums %d, want 1", in(bobs[0]))
	}
	if _, err := b.ActivityDays("bob@h", "#private@h", 260920, 260920); err != ErrUnknown {
		t.Fatalf("a hidden record's range: %v", err)
	}
	for _, bad := range [][2]activity.Date{{260924, 260920}, {260101, 260924}, {260931, 261001}} {
		if _, err := b.ActivityDays("alice@h", "", bad[0], bad[1]); err != ErrRange {
			t.Fatalf("range %v: %v", bad, err)
		}
	}
}

// A restart rebuilds today's ring from the stored days; a database from before
// durable days keeps the ring its queue carried.
func TestTodayIsLoadedBackAtStart(t *testing.T) {
	clock := &testClock{dayAt(24, 9, 0)}
	b, st := daysBus(t, clock)
	b.TickActivity(clock.t)
	b.Send(protocol.Envelope{From: "alice@h", To: "#busy@h", Body: "x"})
	b.Refuse("auth")
	clock.t = dayAt(24, 9, 10)
	b.TickActivity(clock.t)
	saved := stored(t, b, st)
	clock.t = dayAt(24, 11, 0)
	back := New()
	back.Clock(clock.now)
	back.Restore(saved)
	back.Persistence(st)
	back.RestoreActivityDays()
	day, _ := back.Activity("alice@h", "#busy@h")
	if got := day[len(day)-13]; !got.At.Equal(dayAt(24, 9, 0)) || got.In != 1 {
		t.Fatalf("09:00 after restart: %s %+v", got.At, got.Counts)
	}
	all, _ := back.Activity("admin@h", "")
	if all[len(all)-13].Refused != 1 {
		t.Fatalf("node refusal at 09:00 after restart: %d", all[len(all)-13].Refused)
	}

	// Upgrade: no rows yet, the queue's saved ring still restores.
	var total activity.Counts
	ring := activity.Start(dayAt(24, 9, 0), total)
	ring.Tick(dayAt(24, 9, 10), activity.Counts{In: 3})
	legacy := ports.Snapshot{Records: saved.Records, Users: saved.Users, Owner: saved.Owner, OwnerEstablished: true,
		Queues: []ports.Queue{{Name: "#quiet@h", In: 3, Activity: ring.Save(activity.Counts{In: 3})}}}
	up := New()
	up.Clock(clock.now)
	up.Restore(legacy)
	up.Persistence(memory.NewState())
	up.RestoreActivityDays()
	day, _ = up.Activity("alice@h", "#quiet@h")
	if got := day[len(day)-13]; got.In != 3 {
		t.Fatalf("a pre-upgrade ring after start: %+v", got.Counts)
	}
}

// Once a day the boundary drops what is older than KeepDays.
func TestOldDaysArePrunedOncePerDay(t *testing.T) {
	clock := &testClock{dayAt(24, 9, 0)}
	b, st := daysBus(t, clock)
	old := activity.Date(260924).AddDays(-KeepDays - 1)
	keep := activity.Date(260924).AddDays(-KeepDays)
	st.SaveActivityDays([]ports.ActivityDay{{Date: int(old), Name: "#busy@h", Slots: []byte{1}}, {Date: int(keep), Name: "#busy@h", Slots: []byte{1}}})
	b.TickActivity(clock.t)
	r := rows(t, st)["#busy@h"]
	if len(r) != 1 || r[0] != int(keep) {
		t.Fatalf("after the day's first tick: %v, want only %d", r, keep)
	}
}

// On the unfiltered range the daemon Owner's Refused is node-wide, as on the
// day; anyone else's counts only the records they see.
func TestTheOwnersRangeRefusedIsNodeWide(t *testing.T) {
	clock := &testClock{dayAt(24, 9, 0)}
	b, _ := daysBus(t, clock)
	b.TickActivity(clock.t)
	b.Refuse("auth")
	b.Refuse("auth")
	refused := func(caller string) (n int) {
		days, err := b.ActivityDays(caller, "", 260924, 260924)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range days[0].Slots {
			n += s.Refused
		}
		return
	}
	if got := refused("admin@h"); got != 2 {
		t.Fatalf("the owner's range refused %d, want the node's 2", got)
	}
	if got := refused("alice@h"); got != 0 {
		t.Fatalf("alice's range refused %d, want 0", got)
	}
}
