package core

import (
	"time"

	"github.com/parf/ai-agent-bus/internal/activity"
	"github.com/parf/ai-agent-bus/internal/ports"
)

// Counts is one interval's traffic; ActivityPoint is one ten-minute slot of
// the day, when it starts and its counts
// (docs/05-discovery.md#activity-history).
type (
	Counts        = activity.Counts
	ActivityPoint = activity.Slot
)

// totals is the inbox's cumulative counters, which its ring differences at
// each boundary. Nothing on the send or consume path touches the ring.
func (in *inbox) totals() Counts {
	return Counts{In: in.in, Out: in.out, Dropped: in.dropped, Expired: in.expired, Refused: in.refused}
}

// refusedTotal is every refusal this run, the node-wide series the daemon
// Owner sees. Caller holds b.mu.
func (b *Bus) refusedTotal() Counts {
	n := 0
	for _, v := range b.refused {
		n += v
	}
	return Counts{Refused: n}
}

// Clock replaces the time as the bus reads it, for a test that moves it.
func (b *Bus) Clock(now func() time.Time) {
	b.mu.Lock()
	defer b.unlock()
	b.clock = now
}

// TickActivity is driven by the bus process at every minute of the clock;
// each ring closes its slot at :00, :10 … :50 and ignores the rest.
func (b *Bus) TickActivity(now time.Time) {
	b.mu.Lock()
	defer b.unlock()
	for _, in := range b.inboxes {
		in.act.Tick(now, in.totals())
	}
	b.node.Tick(now, b.refusedTotal())
}

func (b *Bus) RecordRefusal(name string) {
	name, err := canon(name)
	if err != nil {
		return
	}
	b.mu.Lock()
	defer b.unlock()
	if in := b.inboxes[name]; in != nil {
		in.refused++
	}
}

// Activity is the last day of one record, or the sum of every live record the
// caller may see: 144 slots, oldest first, the open one last. On the sum the
// daemon Owner's Refused is node-wide.
func (b *Bus) Activity(caller, name string) ([]ActivityPoint, error) {
	if name != "" {
		var err error
		name, err = canon(name)
		if err != nil {
			return nil, err
		}
	}
	b.mu.Lock()
	defer b.unlock()
	if name != "" {
		r, known := b.entity(name)
		if !known || !b.canSee(caller, r) {
			return nil, ErrUnknown
		}
	}
	// Each ring is ticked to now first, so a read between a boundary and the
	// minute tick already has the new slot open, and what arrives after the
	// read is counted in it.
	now := b.clock()
	day := activity.DayAt(now)
	add := func(name string) {
		if in := b.inboxes[name]; in != nil {
			in.act.Tick(now, in.totals())
			day.Add(&in.act, in.totals())
		}
	}
	if name != "" {
		add(name)
		return day.Slots(), nil
	}
	for n, r := range b.records {
		if b.live(r) && b.canSee(caller, r) {
			add(n)
		}
	}
	slots := day.Slots()
	if caller == b.admin {
		node := activity.DayAt(now)
		b.node.Tick(now, b.refusedTotal())
		node.Add(&b.node, b.refusedTotal())
		for i, s := range node.Slots() {
			slots[i].Refused = s.Refused
		}
	}
	return slots, nil
}

// restoreActivity puts a saved ring back, or starts a fresh one when the save
// is missing or damaged — reported, since a lost day is somebody's question.
// ok is false for a damaged one, which the caller saves again so the report
// is made once. Caller holds b.mu.
func (b *Bus) restoreActivity(what string, saved []byte, total Counts) (r activity.Ring, ok bool) {
	r, err := activity.Restore(saved, b.clock(), total)
	if err != nil {
		b.report(ports.Warning, "stored activity of %s is unreadable and starts empty: %v", what, err)
	}
	return r, err == nil
}
