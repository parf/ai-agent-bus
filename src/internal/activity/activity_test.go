package activity

import (
	"testing"
	"time"
	_ "time/tzdata"
)

// Nepal is +05:45, so ten minutes of the node's clock are not ten minutes of
// UTC: a ring aligned to anything but local time lands five minutes off.
var nepal = time.FixedZone("NPT", 5*3600+45*60)

func at(day, h, m int) time.Time { return time.Date(2026, time.September, day, h, m, 0, 0, nepal) }

func dayOf(r *Ring, now time.Time, total Counts) [Slots]Slot {
	d := DayAt(now)
	d.Add(r, total)
	return d.Slots()
}

func in(n int) Counts { return Counts{In: n} }

// run ticks r every ten minutes from from to to, adding one message per slot
// numbered by its local minute of day, so a slot's own count names it.
func run(r *Ring, total *Counts, from, to time.Time) {
	for t := from; !t.After(to); t = t.Add(Span) {
		r.Tick(t, *total)
		total.In += label(t)
	}
}

// label is what run counts in the slot starting at t: 1 + the minute of day.
func label(t time.Time) int { return 1 + t.Hour()*60 + t.Minute() }

func TestFirstSlotClosesAtTheNextTenMinutesOfTheClock(t *testing.T) {
	r := Start(at(23, 10, 7), in(0))
	r.Tick(at(23, 10, 8), in(3))
	r.Tick(at(23, 10, 9).Add(59*time.Second), in(3))
	s := dayOf(&r, at(23, 10, 9), in(3))
	if last := s[Slots-1]; !last.At.Equal(at(23, 10, 0)) || last.In != 3 {
		t.Fatalf("open slot at 10:09 is %s with %d, want 10:00 local with 3", last.At, last.In)
	}
	r.Tick(at(23, 10, 10), in(5))
	s = dayOf(&r, at(23, 10, 10), in(5))
	if closed, open := s[Slots-2], s[Slots-1]; !closed.At.Equal(at(23, 10, 0)) || closed.In != 5 || !open.At.Equal(at(23, 10, 10)) || open.In != 0 {
		t.Fatalf("at 10:10: closed %s=%d, open %s=%d; want 10:00=5 and 10:10=0", closed.At, closed.In, open.At, open.In)
	}
	for i, slot := range s {
		if slot.At.In(nepal).Minute()%10 != 0 || i > 0 && slot.At.Sub(s[i-1].At) != Span {
			t.Fatalf("slot %d starts at %s: slots are ten minutes of the local clock", i, slot.At)
		}
	}
}

// Two days of traffic leave exactly the last 144 slots, each holding its own
// interval: the oldest cell was reused, not kept, not shifted.
func TestRingWrapsAfterADay(t *testing.T) {
	var total Counts
	r := Start(at(21, 0, 0), total)
	run(&r, &total, at(21, 0, 0), at(23, 9, 0))
	s := dayOf(&r, at(23, 9, 0), total)
	if !s[0].At.Equal(at(22, 9, 10)) || !s[Slots-1].At.Equal(at(23, 9, 0)) {
		t.Fatalf("day runs %s to %s, want 22nd 09:10 to 23rd 09:00", s[0].At, s[Slots-1].At)
	}
	for i, slot := range s[:Slots-1] {
		if slot.In != label(slot.At) {
			t.Fatalf("slot %d (%s) holds %d, want %d", i, slot.At, slot.In, label(slot.At))
		}
	}
	if open := s[Slots-1]; open.In != label(at(23, 9, 0)) {
		t.Fatalf("open slot holds %d, want what arrived since 09:00", open.In)
	}
}

// Slots nobody ticked hold a day-old cell; they read zero.
func TestSkippedSlotsReadZero(t *testing.T) {
	var total Counts
	r := Start(at(22, 0, 0), total)
	run(&r, &total, at(22, 0, 0), at(23, 11, 50))
	r.Tick(at(23, 13, 0), total) // quiet, and nobody ticked 12:00 to 12:50
	s := dayOf(&r, at(23, 13, 0), total)
	for i := Slots - 7; i < Slots-1; i++ {
		if s[i].In != 0 {
			t.Fatalf("skipped slot %s holds %d, want 0", s[i].At, s[i].In)
		}
	}
	if s[Slots-8].In != label(at(23, 11, 50)) {
		t.Fatalf("11:50 holds %d, want its own count", s[Slots-8].In)
	}
}

// Ticked again after more than a day, the ring holds nothing of before.
func TestAfterMoreThanADayNothingOldIsShown(t *testing.T) {
	var total Counts
	r := Start(at(20, 0, 0), total)
	run(&r, &total, at(20, 0, 0), at(20, 23, 50))
	r.Tick(at(23, 5, 0), total)
	for _, slot := range dayOf(&r, at(23, 5, 0), total) {
		if slot.In != 0 {
			t.Fatalf("slot %s holds %d from three days ago", slot.At, slot.In)
		}
	}
}

func TestRestartAtThreeKeepsTheMorning(t *testing.T) {
	var total Counts
	r := Start(at(23, 0, 0), total)
	run(&r, &total, at(23, 0, 0), at(23, 14, 50))
	r.Tick(at(23, 15, 0), total)
	saved := r.Save(total)
	back, err := Restore(saved, at(23, 15, 2), in(7)) // a fresh run's counters
	if err != nil {
		t.Fatal(err)
	}
	s := dayOf(&back, at(23, 15, 2), in(7))
	for _, slot := range s[:Slots-1] {
		want := 0
		if slot.At.Day() == 23 {
			want = label(slot.At)
		}
		if slot.In != want {
			t.Fatalf("after restart %s holds %d, want %d", slot.At, slot.In, want)
		}
	}
	if s[Slots-1].In != 0 {
		t.Fatalf("the restored open slot counted %d before anything moved", s[Slots-1].In)
	}
}

// Down from 12:00 to 13:00: six zero slots between an intact 11:50 and the
// open 13:00, whose count is only what the new run sees.
func TestADownHourIsSixZeroSlots(t *testing.T) {
	var total Counts
	r := Start(at(22, 0, 0), total)
	run(&r, &total, at(22, 0, 0), at(23, 12, 0))
	saved := r.Save(total) // at 12:00:00, the 12:00 slot counted so far
	back, err := Restore(saved, at(23, 13, 0).Add(10*time.Second), in(100))
	if err != nil {
		t.Fatal(err)
	}
	s := dayOf(&back, at(23, 13, 5), in(104))
	if s[Slots-1].In != 4 {
		t.Fatalf("open 13:00 holds %d, want the 4 counted since restart", s[Slots-1].In)
	}
	if got := s[Slots-7]; !got.At.Equal(at(23, 12, 0)) || got.In != label(at(23, 12, 0)) {
		t.Fatalf("12:00 holds %d, want what it counted before the stop", got.In)
	}
	for i := Slots - 6; i < Slots-1; i++ {
		if s[i].In != 0 {
			t.Fatalf("down slot %s holds %d, want 0", s[i].At, s[i].In)
		}
	}
	if s[Slots-8].In != label(at(23, 11, 50)) || s[0].In != label(s[0].At) {
		t.Fatal("the slots before the stop were not restored")
	}
	// Down for more than a day, nothing old comes back.
	late, _ := Restore(saved, at(24, 18, 0), Counts{})
	for _, slot := range dayOf(&late, at(24, 18, 0), Counts{}) {
		if slot.In != 0 {
			t.Fatalf("slot %s older than a day came back: %d", slot.At, slot.In)
		}
	}
	// Back inside the slot that was open, it keeps what that slot counted.
	same, _ := Restore(saved, at(23, 12, 4), in(0))
	if s := dayOf(&same, at(23, 12, 4), in(2)); s[Slots-1].In != label(at(23, 12, 0))+2 {
		t.Fatalf("restart inside the open slot: %d", s[Slots-1].In)
	}
	if _, err := Restore([]byte{format, 9}, at(23, 12, 0), Counts{}); err != ErrDamaged {
		t.Fatalf("a damaged save restored: %v", err)
	}
}

func TestSpringForwardLeavesSixZeroSlots(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	before := time.Date(2026, time.March, 8, 1, 50, 0, 0, ny)
	r := Start(before.Add(-Span), in(0))
	r.Tick(before, in(1))
	after := before.Add(Span) // the clock reads 03:00
	if after.Hour() != 3 {
		t.Fatalf("fixture: %s", after)
	}
	r.Tick(after, in(4))
	s := dayOf(&r, after, in(4))
	if s[Slots-9].In != 1 || s[Slots-8].In != 3 {
		t.Fatalf("01:40 and 01:50 hold %d and %d, want 1 and 3", s[Slots-9].In, s[Slots-8].In)
	}
	for i := Slots - 7; i < Slots; i++ {
		if s[i].In != 0 {
			t.Fatalf("skipped slot %d holds %d", i, s[i].In)
		}
	}
}

// The hour a fall back repeats: the second pass overwrites the first, and
// the first pass never shows up in yesterday's cells it shares.
func TestFallBackOverwritesTheRepeatedHour(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	var total Counts
	start := time.Date(2026, time.October, 31, 1, 0, 0, 0, ny)
	r := Start(start, total)
	tick := func(t time.Time, n int) { r.Tick(t, total); total.In += n }
	for t := start; t.Before(start.Add(24 * time.Hour)); t = t.Add(Span) {
		tick(t, 1) // the day before: one per slot
	}
	first := time.Date(2026, time.November, 1, 1, 0, 0, 0, ny) // EDT
	for i := 0; i < 6; i++ {
		tick(first.Add(time.Duration(i)*Span), 50)
	}
	second := first.Add(time.Hour) // 01:00 EST
	if second.Hour() != 1 {
		t.Fatalf("fixture: %s", second)
	}
	tick(second, 7)
	s := dayOf(&r, second, total)
	for i := 0; i < 5; i++ {
		if s[i].In == 50 {
			t.Fatalf("slot %d (%s) shows the first pass in yesterday's place", i, s[i].At)
		}
	}
	for i := 1; i < 6; i++ {
		tick(second.Add(time.Duration(i)*Span), 7)
	}
	two := second.Add(time.Hour)
	tick(two, 0)
	s = dayOf(&r, two, total)
	for i := Slots - 7; i < Slots-1; i++ {
		if s[i].In != 7 || s[i].At.Hour() != 1 {
			t.Fatalf("repeated slot %s holds %d, want the second pass's 7", s[i].At, s[i].In)
		}
	}
}

// A clock that steps back moves nothing; what arrives meanwhile stays in the
// open slot and closes with it when the clock is past it again.
func TestClockSteppingBackKeepsTheOpenSlot(t *testing.T) {
	r := Start(at(23, 10, 5), in(0))
	r.Tick(at(23, 10, 15), in(1))
	r.Tick(at(23, 10, 5), in(2))
	if s := dayOf(&r, at(23, 10, 15), in(2)); s[Slots-1].In != 1 || s[Slots-2].In != 1 || !s[Slots-1].At.Equal(at(23, 10, 10)) {
		t.Fatalf("after stepping back: 10:00=%d, open %s=%d; want 1 and 10:10=1", s[Slots-2].In, s[Slots-1].At, s[Slots-1].In)
	}
	r.Tick(at(23, 10, 20), in(3))
	if s := dayOf(&r, at(23, 10, 20), in(3)); s[Slots-2].In != 2 || s[Slots-3].In != 1 {
		t.Fatalf("10:10 closed with %d, want 2", s[Slots-2].In)
	}
}

func TestDaySumsRings(t *testing.T) {
	a := Start(at(23, 9, 0), Counts{})
	b := Start(at(23, 9, 0), Counts{Out: 10})
	a.Tick(at(23, 9, 10), Counts{In: 2, Refused: 1})
	b.Tick(at(23, 9, 10), Counts{Out: 13, Dropped: 1})
	d := DayAt(at(23, 9, 12))
	d.Add(&a, Counts{In: 3, Refused: 1})
	d.Add(&b, Counts{Out: 14, Dropped: 1, Expired: 2})
	s := d.Slots()
	if got := s[Slots-2].Counts; got != (Counts{In: 2, Out: 3, Dropped: 1, Refused: 1}) {
		t.Fatalf("09:00 sums to %+v", got)
	}
	if got := s[Slots-1].Counts; got != (Counts{In: 1, Out: 1, Expired: 2}) {
		t.Fatalf("open 09:10 sums to %+v", got)
	}
}
