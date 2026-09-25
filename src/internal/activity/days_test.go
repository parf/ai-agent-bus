package activity

import (
	"testing"
	"time"
)

func TestDateIsYymmddOfTheLocalDay(t *testing.T) {
	// 23:30 in Nepal is still the 23rd there, the 23rd 17:45 UTC.
	if got := DateOf(at(23, 23, 30)); got != 260923 {
		t.Fatalf("DateOf = %d, want 260923", got)
	}
	if d := Date(260930).AddDays(1); d != 261001 {
		t.Fatalf("260930 + 1 day = %d", d)
	}
	if d := Date(260301).AddDays(-1); d != 260228 {
		t.Fatalf("260301 - 1 day = %d", d)
	}
	for _, bad := range []Date{0, 261301, 260231, 260900} {
		if bad.Valid() {
			t.Fatalf("%d reads as a real day", bad)
		}
	}
	if !Date(260924).Valid() {
		t.Fatal("260924 is a real day")
	}
}

func TestADayRoundTripsAndOnlyItsNonEmptySlotsAreStored(t *testing.T) {
	var s DaySlots
	s[0] = Counts{In: 1}
	s[77] = Counts{In: 300, Out: 299, Dropped: 1, Expired: 2, Refused: 3}
	s[143] = Counts{Refused: 9}
	b := EncodeDay(&s)
	got, err := DecodeDay(b)
	if err != nil || got != s {
		t.Fatalf("round trip: %v\n got  %v\n want %v", err, got, s)
	}
	// A full day of one message per slot stays small: sparse, then zstd.
	var full DaySlots
	for i := range full {
		full[i] = Counts{In: 1, Out: 1}
	}
	if n := len(EncodeDay(&full)); n > 64 {
		t.Fatalf("a full day takes %d bytes", n)
	}
}

func TestAnEmptyDayHasNoRow(t *testing.T) {
	var s DaySlots
	if b := EncodeDay(&s); b != nil {
		t.Fatalf("an empty day encodes to %d bytes", len(b))
	}
}

func TestADamagedDayIsRefused(t *testing.T) {
	var s DaySlots
	s[5] = Counts{In: 2}
	good := EncodeDay(&s)
	for name, b := range map[string][]byte{"not zstd": {1, 2, 3}, "empty": {}, "truncated": good[:len(good)-2]} {
		if _, err := DecodeDay(b); err == nil {
			t.Fatalf("%s decoded", name)
		}
	}
}

func TestARingSpanningMidnightIsTwoDays(t *testing.T) {
	var total Counts
	r := Start(at(23, 23, 40), total)
	run(&r, &total, at(23, 23, 40), at(24, 0, 10))
	days := r.Days(total)
	if len(days) != 2 || days[260923] == nil || days[260924] == nil {
		t.Fatalf("days = %v", keys(days))
	}
	if got := days[260923][142].In; got != label(at(23, 23, 40)) {
		t.Fatalf("23:40 slot = %d", got)
	}
	if got := days[260924][0].In; got != label(at(24, 0, 0)) {
		t.Fatalf("00:00 slot = %d", got)
	}
	// Silent slots are not kept.
	if c := days[260924][5]; c != (Counts{}) {
		t.Fatalf("00:50 = %v", c)
	}
}

func TestTickClosingNamesTheRowOnlyWhenTheClosedSlotCounted(t *testing.T) {
	total := Counts{}
	r := Start(at(23, 10, 0), total)
	if _, write := r.TickClosing(at(23, 10, 5), total); write {
		t.Fatal("a tick inside the slot closed nothing")
	}
	total.In = 3
	d, write := r.TickClosing(at(23, 10, 10), total)
	if !write || d.Date != 260923 {
		t.Fatalf("closing a slot with counts: %d %v", d, write)
	}
	if _, write := r.TickClosing(at(23, 10, 20), total); write {
		t.Fatal("closing a silent slot wrote a row")
	}
	// The slot that closes at midnight writes the day it belongs to.
	r2 := Start(at(23, 23, 50), Counts{})
	d, write = r2.TickClosing(at(24, 0, 0), Counts{Out: 1})
	if !write || d.Date != 260923 {
		t.Fatalf("23:50 closing at midnight: %d %v", d, write)
	}
}

func TestARingRebuiltFromItsDaysReadsTheSame(t *testing.T) {
	var total Counts
	r := Start(at(23, 9, 0), total)
	run(&r, &total, at(23, 9, 0), at(24, 8, 50))
	now := at(24, 8, 55)
	want := dayOf(&r, now, total)
	back := FromDays(r.Days(total), now, total)
	if got := dayOf(&back, now, total); !equal(got, want) {
		t.Fatal("rebuilt ring reads differently")
	}
	// A day older than the last 24 hours is not put back.
	old := map[Date]*DaySlots{260920: {Counts{In: 5}}}
	fresh := FromDays(old, now, Counts{})
	for _, s := range dayOf(&fresh, now, Counts{}) {
		if s.In != 0 {
			t.Fatal("a day outside the ring was restored")
		}
	}
}

func TestCoversIsTheRingsWindow(t *testing.T) {
	r := Start(at(24, 8, 0), Counts{})
	r.Tick(at(24, 12, 0), Counts{})
	if !r.Covers(260924, 8*6) || !r.Covers(260924, 12*6) {
		t.Fatal("slots inside the ring are not covered")
	}
	if r.Covers(260924, 12*6+1) || r.Covers(260923, 8*6) {
		t.Fatal("slots outside the ring are covered")
	}
}

func keys(m map[Date]*DaySlots) []Date {
	var out []Date
	for k := range m {
		out = append(out, k)
	}
	return out
}

func equal(a, b []Slot) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].At.Equal(b[i].At) || a[i].Counts != b[i].Counts {
			return false
		}
	}
	return true
}

var _ = time.Second

func TestASilentRingHoldsNoDays(t *testing.T) {
	r := Start(at(23, 9, 0), Counts{})
	r.Tick(at(24, 8, 0), Counts{})
	if d := r.Days(Counts{}); len(d) != 0 {
		t.Fatalf("a silent ring holds days %v", keys(d))
	}
}

// A ledger holds a day whole whatever the ring still covers: the slot at
// midnight survives the next day's ticks, and a jump of a day or more still
// hands back the slot it closed, never an empty day in its place.
func TestALedgerKeepsWholeDays(t *testing.T) {
	var l Ledger
	var total Counts
	r := Start(at(23, 0, 0), total)
	total.In = 1 // 00:00 on the 23rd
	c, ok := r.TickClosing(at(23, 0, 10), total)
	if !ok || c.Date != 260923 || c.Index != 0 || c.Counts.In != 1 {
		t.Fatalf("the 00:00 slot closed as %+v %v", c, ok)
	}
	l.Set(c)
	// A whole day on, the ring no longer covers the 23rd's 00:00; the ledger does.
	r.Tick(at(23, 23, 50), total)
	total.In = 2 // in the 23:50 slot
	c, _ = r.TickClosing(at(24, 0, 0), total)
	day := l.Set(c)
	if c.Date != 260923 || c.Index != 143 || day[0].In != 1 || day[143].In != 1 {
		t.Fatalf("the 23rd after midnight: slot 0 %v, slot 143 %v (closed %+v)", day[0], day[143], c)
	}
	// A jump of exactly two days reuses the closed slot's cell: its counts
	// must be taken before the tick, and it is still no empty day.
	total.In = 5
	c, ok = r.TickClosing(at(26, 0, 0), total)
	if !ok || c.Date != 260924 || c.Index != 0 || c.Counts.In != 3 {
		t.Fatalf("after a two-day jump: %+v %v", c, ok)
	}
	day = l.Set(c)
	if day.Empty() {
		t.Fatal("a jump left an empty day to write")
	}
	// Only the latest day and the one before are kept.
	l.Set(Closed{Date: 260926, Index: 1, Counts: Counts{In: 1}})
	if len(l.Days(Closed{})) != 1 {
		t.Fatalf("days kept: %d", len(l.Days(Closed{})))
	}
}

// A stop writes the ledger's days with the open slot set into a copy; the
// ledger itself is not changed by it.
func TestAStopWritesTheOpenSlotWithoutKeepingIt(t *testing.T) {
	var l Ledger
	l.Seed(260923, DaySlots{5: {In: 2}})
	open := Closed{Date: 260924, Index: 60, Counts: Counts{Out: 1}}
	days := l.Days(open)
	if days[260923][5].In != 2 || days[260924][60].Out != 1 {
		t.Fatalf("stop days: %v", days)
	}
	if again := l.Days(Closed{}); len(again) != 1 {
		t.Fatalf("the open slot stayed in the ledger: %v", again)
	}
}
