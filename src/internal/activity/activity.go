// Package activity keeps one record's last day of traffic as a ring of 144
// ten-minute slots on the node's clock: slot 0 is 00:00–00:10 local time,
// slot 143 is 23:50–24:00 (docs/05-discovery.md#activity-history).
//
// Outside, there are three moves: Start or Restore a Ring, Tick it with the
// cumulative counters the caller already keeps, and read a Day that sums any
// number of rings. Pointers, slot arithmetic, clock steps and daylight saving
// stay in here. A slot carries its counts and nothing else: a time nobody
// counted — the daemon down, an hour daylight saving skipped — reads as zero.
//
// Nothing here reads the clock: every call is given its now.
package activity

import (
	"encoding/binary"
	"errors"
	"time"
)

// Slots is how many slots one day holds, and Span how long each one is.
const (
	Slots = 144
	Span  = 10 * time.Minute
)

// Counts is one interval's traffic, or — handed to Tick, Save, Restore and
// Add — a record's cumulative counters, which only grow within a run.
type Counts struct {
	In      int `json:"in"`
	Out     int `json:"out"`
	Dropped int `json:"dropped"`
	Expired int `json:"expired"`
	Refused int `json:"refused"`
}

func (c *Counts) add(o Counts) {
	c.In += o.In
	c.Out += o.Out
	c.Dropped += o.Dropped
	c.Expired += o.Expired
	c.Refused += o.Refused
}

// since is c − o, never below zero: a counter that went back is a restart the
// caller did not tell us about, and it counted nothing.
func (c Counts) since(o Counts) Counts {
	return Counts{max(0, c.In-o.In), max(0, c.Out-o.Out), max(0, c.Dropped-o.Dropped), max(0, c.Expired-o.Expired), max(0, c.Refused-o.Refused)}
}

// Ring is one record's day. The zero Ring is empty and ticks from its first
// Tick; Start and Restore are how a caller means to begin.
//
// Its slots are addressed by a civil slot number — days since 1970-01-01 of
// the local date, times 144, plus the slot of that day — so a daylight-saving
// change moves the number the way the wall clock does, and never by an
// hour of elapsed time. end is the open slot; start the oldest slot the ring
// holds; slots outside [start, end] read as zero, and end−start < Slots.
type Ring struct {
	cells      [Slots]Counts
	start, end int64
	// base is the cumulative counters at the last boundary: the open slot is
	// cells[end] plus whatever moved since.
	base Counts
	// at is the latest instant ticked, in Unix seconds, to tell a clock that
	// stepped back from a daylight-saving fall.
	at int64
}

// Start begins a ring whose open slot is now's, counting from total.
func Start(now time.Time, total Counts) Ring {
	k := slot(now)
	return Ring{start: k, end: k, base: total, at: now.Unix()}
}

// Tick moves the ring to now: the open slot closes with what total moved
// since the last boundary, the slots nobody ticked read zero, and after a
// day the oldest slot is reused. Within the open slot it does nothing, so it
// may be called as often as liked. A clock that steps back moves nothing:
// the counts stay in the open slot until the clock catches up. A
// daylight-saving fall repeats an hour, and the second pass overwrites the
// first.
func (r *Ring) Tick(now time.Time, total Counts) {
	if r.at == 0 && r.end == 0 {
		*r = Start(now, total)
		return
	}
	u := now.Unix()
	if u < r.at {
		return
	}
	r.at = u
	k := slot(now)
	if k == r.end {
		return
	}
	r.cells[cell(r.end)].add(total.since(r.base))
	r.base = total
	r.move(k)
}

// move makes k the open slot, emptying it and every slot skipped on the way.
func (r *Ring) move(k int64) {
	switch {
	case k-r.end >= Slots:
		// More than a day away: nothing held is inside the day any more.
		r.start = k
	case k > r.end:
		for j := r.end + 1; j < k; j++ {
			r.cells[cell(j)] = Counts{}
		}
		r.start = max(r.start, k-(Slots-1))
	default:
		// Back, while time went forward: a daylight-saving fall, or a
		// restore behind the save. The slots after k are the pass being
		// repeated, beyond end and so unread; the ones a day before them
		// share their cells and are already before start.
		r.start = min(r.start, k)
	}
	r.end = k
	r.cells[cell(k)] = Counts{}
}

// cell is where civil slot k lives in the ring.
func cell(k int64) int { return int((k%Slots + Slots) % Slots) }

// Day is the last day of one ring or the sum of several, oldest slot first,
// ending with the slot open at the time it was made for.
type Day struct {
	slots [Slots]Counts
	end   int64
	loc   *time.Location
}

// DayAt is an empty day whose last slot is now's.
func DayAt(now time.Time) Day { return Day{end: slot(now), loc: now.Location()} }

// Add sums a ring into the day. total is the ring's cumulative counters now,
// which is what the open slot shows beyond its last boundary.
func (d *Day) Add(r *Ring, total Counts) {
	first := d.end - (Slots - 1)
	for k := max(r.start, first); k <= min(r.end, d.end); k++ {
		c := r.cells[cell(k)]
		if k == r.end {
			c.add(total.since(r.base))
		}
		d.slots[k-first].add(c)
	}
}

// Slot is one ten-minute interval of a Day: when it starts, and its counts.
type Slot struct {
	At time.Time `json:"at"`
	Counts
}

// Slots is the day, oldest first; the last one is the slot still open.
func (d *Day) Slots() [Slots]Slot {
	var out [Slots]Slot
	for i := range out {
		out[i] = Slot{At: slotTime(d.end-(Slots-1)+int64(i), d.loc), Counts: d.slots[i]}
	}
	return out
}

// slot is now's civil slot number.
func slot(now time.Time) int64 {
	y, m, dd := now.Date()
	h, mi, _ := now.Clock()
	days := time.Date(y, m, dd, 0, 0, 0, 0, time.UTC).Unix() / 86400
	return days*Slots + int64(h*60+mi)/int64(Span/time.Minute)
}

// slotTime is when civil slot k starts, in loc. A slot inside an hour that
// daylight saving skipped has no such local time; Go's normalisation names
// the hour after it.
func slotTime(k int64, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	day, s := k/Slots, int(k%Slots)
	if s < 0 {
		day, s = day-1, s+Slots
	}
	y, m, d := time.Unix(day*86400, 0).UTC().Date()
	minutes := s * int(Span/time.Minute)
	return time.Date(y, m, d, minutes/60, minutes%60, 0, 0, loc)
}

// Save is the ring as it stands, the open slot's counts so far included, in
// a form only Restore reads. total is the cumulative counters now.
func (r *Ring) Save(total Counts) []byte {
	b := make([]byte, 0, 16+8*int(r.end-r.start+1))
	b = append(b, format)
	b = binary.AppendVarint(b, r.start)
	b = binary.AppendVarint(b, r.end)
	for k := r.start; k <= r.end; k++ {
		c := r.cells[cell(k)]
		if k == r.end {
			c.add(total.since(r.base))
		}
		for _, v := range [...]int{c.In, c.Out, c.Dropped, c.Expired, c.Refused} {
			b = binary.AppendUvarint(b, uint64(v))
		}
	}
	return b
}

const format = 1

// ErrDamaged is a saved ring Restore cannot read.
var ErrDamaged = errors.New("activity ring is damaged")

// Restore puts a saved ring back at now, counting from total. What is older
// than a day is dropped and the time between the save and now reads zero; a
// restart inside the slot that was open keeps what that slot had counted. An
// empty save is a fresh ring. A damaged one is a fresh ring and ErrDamaged.
func Restore(data []byte, now time.Time, total Counts) (Ring, error) {
	r := Start(now, total)
	if len(data) == 0 {
		return r, nil
	}
	saved, err := decode(data)
	if err != nil {
		return r, err
	}
	saved.base, saved.at = total, now.Unix()
	if k := slot(now); k != saved.end {
		saved.move(k)
	}
	return saved, nil
}

func decode(data []byte) (Ring, error) {
	var r Ring
	if data[0] != format {
		return r, ErrDamaged
	}
	p := 1
	next := func() (int64, bool) {
		v, n := binary.Varint(data[p:])
		p += max(n, 0)
		return v, n > 0
	}
	start, ok1 := next()
	end, ok2 := next()
	if !ok1 || !ok2 || end < start || end-start >= Slots {
		return r, ErrDamaged
	}
	r.start, r.end = start, end
	for k := start; k <= end; k++ {
		var v [5]int
		for i := range v {
			u, n := binary.Uvarint(data[p:])
			if n <= 0 || u > 1<<62 {
				return r, ErrDamaged
			}
			p += n
			v[i] = int(u)
		}
		r.cells[cell(k)] = Counts{v[0], v[1], v[2], v[3], v[4]}
	}
	if p != len(data) {
		return r, ErrDamaged
	}
	return r, nil
}
