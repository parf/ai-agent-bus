package activity

// Durable days: a ring holds the last 24 hours; what outlives it is one row
// per record per local calendar day, keyed by the date as yymmdd, holding
// only that day's non-empty slots, sparse and zstd-compressed
// (docs/05-discovery.md#activity-history). A day with no traffic has no row.

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

// Date is a local calendar day as yymmdd: 260924 is 2026-09-24. It orders as
// the days do within a century, which is what a range query needs.
type Date int

// DateOf is t's local calendar day.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date((y%100)*10000 + int(m)*100 + d)
}

// Time is the date's midnight in loc.
func (d Date) Time(loc *time.Location) time.Time {
	return time.Date(2000+int(d)/10000, time.Month(int(d)/100%100), int(d)%100, 0, 0, 0, 0, loc)
}

// AddDays is the date n days later, n < 0 for earlier.
func (d Date) AddDays(n int) Date { return DateOf(d.Time(time.UTC).AddDate(0, 0, n)) }

// Valid says d names a real calendar day.
func (d Date) Valid() bool { return d > 0 && DateOf(d.Time(time.UTC)) == d }

// civil is the civil day number the ring's slot numbers are built from.
func (d Date) civil() int64 { return d.Time(time.UTC).Unix() / 86400 }

func dateOfCivil(day int64) Date { return DateOf(time.Unix(day*86400, 0).UTC()) }

// DaySlots is one calendar day's 144 slots, slot 0 at 00:00.
type DaySlots [Slots]Counts

// Empty says nothing was counted all day.
func (s *DaySlots) Empty() bool {
	for _, c := range s {
		if c != (Counts{}) {
			return false
		}
	}
	return true
}

// Add sums o into s.
func (s *DaySlots) Add(o *DaySlots) {
	for i := range s {
		s[i].add(o[i])
	}
}

// Days is every calendar day the ring holds, the open slot's counts so far
// included: one or two dates, since a ring spans at most a day. total is the
// cumulative counters now.
func (r *Ring) Days(total Counts) map[Date]*DaySlots {
	out := map[Date]*DaySlots{}
	for k := r.start; k <= r.end; k++ {
		c := r.cells[cell(k)]
		if k == r.end {
			c.add(total.since(r.base))
		}
		if c == (Counts{}) {
			continue
		}
		d := dateOfCivil(floorDiv(k, Slots))
		if out[d] == nil {
			out[d] = &DaySlots{}
		}
		out[d][int(k-floorDiv(k, Slots)*Slots)] = c
	}
	return out
}

// Closed is the slot a Tick closed: its day, its index in the day and what
// it counted.
type Closed struct {
	Date   Date
	Index  int
	Counts Counts
}

// TickClosing is Tick that also hands back the slot it closed, when it
// counted anything: what a durable day adds, and nothing else needs writing.
// Its counts are taken before the tick, since a jump of a day or more reuses
// the closed slot's cell.
func (r *Ring) TickClosing(now time.Time, total Counts) (Closed, bool) {
	fresh := r.at == 0 && r.end == 0
	prev := r.end
	c := r.cells[cell(prev)]
	c.add(total.since(r.base))
	r.Tick(now, total)
	if fresh || r.end == prev || c == (Counts{}) {
		return Closed{}, false
	}
	day := floorDiv(prev, Slots)
	return Closed{Date: dateOfCivil(day), Index: int(prev - day*Slots), Counts: c}, true
}

// Open is the open slot so far: its day, index and counts.
func (r *Ring) Open(total Counts) Closed {
	c := r.cells[cell(r.end)]
	c.add(total.since(r.base))
	day := floorDiv(r.end, Slots)
	return Closed{Date: dateOfCivil(day), Index: int(r.end - day*Slots), Counts: c}
}

// Ledger is one name's durable days in progress: every closed slot is set
// into its date, so a day's row is whole whatever the ring still holds. It
// keeps the day of the latest slot and the one before, which is all a
// boundary or a stop can still change.
type Ledger struct {
	days map[Date]*DaySlots
}

// Seed puts a stored day back, as the start reads it.
func (l *Ledger) Seed(d Date, s DaySlots) {
	if l.days == nil {
		l.days = map[Date]*DaySlots{}
	}
	l.days[d] = &s
}

// Set records a closed slot and drops days older than the one before it. A
// daylight-saving fall repeats a slot, and the second pass overwrites the
// first, as the ring does.
func (l *Ledger) Set(c Closed) *DaySlots {
	if l.days == nil {
		l.days = map[Date]*DaySlots{}
	}
	s := l.days[c.Date]
	if s == nil {
		s = &DaySlots{}
		l.days[c.Date] = s
	}
	s[c.Index] = c.Counts
	for d := range l.days {
		if d < c.Date.AddDays(-1) {
			delete(l.days, d)
		}
	}
	return s
}

// Days is every day the ledger holds, with the open slot's counts so far set
// into a copy: what a stop writes. The ledger itself is not changed.
func (l *Ledger) Days(open Closed) map[Date]DaySlots {
	out := map[Date]DaySlots{}
	for d, s := range l.days {
		out[d] = *s
	}
	if open.Counts != (Counts{}) {
		s := out[open.Date]
		s[open.Index] = open.Counts
		out[open.Date] = s
	}
	return out
}

// Day is the ring's slots of one calendar day, open slot included; days it
// does not hold read zero.
func (r *Ring) Day(d Date, total Counts) DaySlots {
	var out DaySlots
	if s := r.Days(total)[d]; s != nil {
		out = *s
	}
	return out
}

// Covers says slot i of day d is inside the ring, so the ring, not a stored
// row, is the truth for it.
func (r *Ring) Covers(d Date, i int) bool {
	k := d.civil()*Slots + int64(i)
	return k >= r.start && k <= r.end
}

// FromDays is a ring at now, counting from total, holding the stored days'
// slots that fall inside the last day. The open slot keeps what it had
// counted when it was saved, as Restore does.
func FromDays(days map[Date]*DaySlots, now time.Time, total Counts) Ring {
	r := Start(now, total)
	r.start = r.end - (Slots - 1)
	for d, s := range days {
		base := d.civil() * Slots
		for i, c := range s {
			if k := base + int64(i); k >= r.start && k <= r.end {
				r.cells[cell(k)] = c
			}
		}
	}
	return r
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

const dayFormat = 1

// ErrDamagedDay is a stored day DecodeDay cannot read.
var ErrDamagedDay = errors.New("stored activity day is damaged")

var (
	encOnce sync.Once
	enc     *zstd.Encoder
	dec     *zstd.Decoder
)

func codec() {
	encOnce.Do(func() {
		enc, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBetterCompression), zstd.WithEncoderConcurrency(1))
		dec, _ = zstd.NewReader(nil, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(1<<20))
	})
}

// EncodeDay is the day as stored: only its non-empty slots, each as the gap
// since the previous one and its five counts, zstd-compressed — a steady day
// is one repeated pattern, which zstd reduces to almost nothing. An empty day is nil: it has no row.
func EncodeDay(s *DaySlots) []byte {
	if s.Empty() {
		return nil
	}
	b := []byte{dayFormat}
	last := -1
	for i, c := range s {
		if c == (Counts{}) {
			continue
		}
		b = binary.AppendUvarint(b, uint64(i-last-1))
		last = i
		for _, v := range [...]int{c.In, c.Out, c.Dropped, c.Expired, c.Refused} {
			b = binary.AppendUvarint(b, uint64(v))
		}
	}
	codec()
	return enc.EncodeAll(b, nil)
}

// DecodeDay reads a stored day back.
func DecodeDay(data []byte) (DaySlots, error) {
	var s DaySlots
	codec()
	b, err := dec.DecodeAll(data, nil)
	if err != nil || len(b) == 0 || b[0] != dayFormat {
		return s, ErrDamagedDay
	}
	p := 1
	next := func() (uint64, bool) {
		v, n := binary.Uvarint(b[p:])
		if n <= 0 || v > 1<<62 {
			return 0, false
		}
		p += n
		return v, true
	}
	last := -1
	for p < len(b) {
		gap, ok := next()
		i := last + 1 + int(gap)
		if !ok || gap >= Slots || i >= Slots {
			return DaySlots{}, ErrDamagedDay
		}
		last = i
		var v [5]int
		for j := range v {
			u, ok := next()
			if !ok {
				return DaySlots{}, ErrDamagedDay
			}
			v[j] = int(u)
		}
		s[i] = Counts{v[0], v[1], v[2], v[3], v[4]}
	}
	return s, nil
}

// SlotsOf is a stored or summed day as a Day reads: 144 slots, 00:00 first,
// each with the local time it starts.
func SlotsOf(d Date, s *DaySlots, loc *time.Location) []Slot {
	out := make([]Slot, Slots)
	base := d.civil() * Slots
	for i := range out {
		out[i] = Slot{At: slotTime(base+int64(i), loc), Counts: s[i]}
	}
	return out
}
