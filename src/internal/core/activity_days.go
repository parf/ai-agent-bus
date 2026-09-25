package core

import (
	"errors"
	"sort"
	"time"

	"github.com/parf/ai-agent-bus/internal/activity"
	"github.com/parf/ai-agent-bus/internal/ports"
)

// Durable days (docs/05-discovery.md#activity-history): each name's traffic
// per local calendar day, beyond the ring's last 24 hours. A row is written
// when a slot that counted something closes — once per ten minutes at most,
// never on the send or consume path — and at a graceful stop, and read back
// at start and for a range.

const (
	// KeepDays is how long a day is kept: past a month view and a year's lookback.
	KeepDays = 400
	// RangeDays is the most one range read spans: a month and then some.
	RangeDays = 62
	// nodeSeries is the row name of the node's own refusals.
	nodeSeries = ""
)

// ErrRange is a range the daemon does not read: a date that is not one, an
// end before its start, or longer than RangeDays.
var ErrRange = errors.New("activity range must be real dates, from before to, at most 62 days")

// ActivityDay is one calendar day of a name, or of every name the caller may
// see: its 144 slots, 00:00 first.
type ActivityDay struct {
	Date  activity.Date
	Slots []ActivityPoint
}

func (b *Bus) activityStore() ports.ActivityStore {
	if as, ok := b.store.(ports.ActivityStore); ok {
		return as
	}
	return nil
}

// closing ticks every ring to now and gathers the rows a boundary changed:
// each slot that closed with counts in it is set into its name's ledger, and
// that day's row is the ledger's day, whole. A day is never written empty
// from here: nothing counted means nothing to write. Caller holds b.mu.
func (b *Bus) closing(now time.Time) []ports.ActivityDay {
	var rows []ports.ActivityDay
	for name, in := range b.inboxes {
		if c, write := in.act.TickClosing(now, in.totals()); write {
			if _, known := b.records[name]; known {
				rows = append(rows, ports.ActivityDay{Date: int(c.Date), Name: name, Slots: activity.EncodeDay(in.days.Set(c))})
			}
		}
	}
	if c, write := b.node.TickClosing(now, b.refusedTotal()); write {
		rows = append(rows, ports.ActivityDay{Date: int(c.Date), Name: nodeSeries, Slots: activity.EncodeDay(b.nodeDays.Set(c))})
	}
	return rows
}

// writeDays saves rows, and once a day drops what is older than KeepDays.
// Called without b.mu: a database write never holds the bus.
func (b *Bus) writeDays(as ports.ActivityStore, rows []ports.ActivityDay, today activity.Date, prune bool) {
	if len(rows) > 0 {
		if err := as.SaveActivityDays(rows); err != nil {
			b.report(ports.Error, "activity days not saved: %v", err)
		}
	}
	if prune {
		if err := as.PruneActivity(int(Oldest(today))); err != nil {
			b.report(ports.Error, "old activity days not pruned: %v", err)
		}
	}
}

// Oldest is the first day kept on today: KeepDays days, today included.
func Oldest(today activity.Date) activity.Date { return today.AddDays(-(KeepDays - 1)) }

// dayRows is every day a ledger holds with the open slot so far: what a stop
// writes, empty days left out.
func dayRows(name string, l *activity.Ledger, open activity.Closed) []ports.ActivityDay {
	var rows []ports.ActivityDay
	for d, s := range l.Days(open) {
		if b := activity.EncodeDay(&s); b != nil {
			rows = append(rows, ports.ActivityDay{Date: int(d), Name: name, Slots: b})
		}
	}
	return rows
}

// FlushActivity writes every day in progress, the open slot's counts so far
// included: the graceful stop's, so a restart inside a slot keeps them.
func (b *Bus) FlushActivity() error {
	b.mu.Lock()
	as := b.activityStore()
	if as == nil {
		b.unlock()
		return nil
	}
	var rows []ports.ActivityDay
	for name, in := range b.inboxes {
		if _, known := b.records[name]; known {
			rows = append(rows, dayRows(name, &in.days, in.act.Open(in.totals()))...)
		}
	}
	rows = append(rows, dayRows(nodeSeries, &b.nodeDays, b.node.Open(b.refusedTotal()))...)
	b.unlock()
	if len(rows) == 0 {
		return nil
	}
	return as.SaveActivityDays(rows)
}

// RestoreActivityDays rebuilds each ring and each ledger from the stored days
// of yesterday and today, after the store is bound. A name without rows keeps
// the ring its queue carried — how a database from before durable days
// starts — and that ring's days are written at once, so the first minute's
// queue flush, which no longer carries rings, cannot lose them. A damaged row
// is reported and read as nothing.
func (b *Bus) RestoreActivityDays() {
	b.mu.Lock()
	as := b.activityStore()
	now := b.clock()
	b.unlock()
	if as == nil {
		return
	}
	today := activity.DateOf(now)
	stored, err := as.ActivityDays(int(today.AddDays(-1)), int(today), nil)
	if err != nil {
		b.report(ports.Error, "activity days not read at start: %v", err)
		return
	}
	byName := map[string]map[activity.Date]*activity.DaySlots{}
	for _, r := range stored {
		s, err := activity.DecodeDay(r.Slots)
		if err != nil {
			b.report(ports.Warning, "stored activity of %s on %d is unreadable and reads as nothing: %v", nameOr(r.Name), r.Date, err)
			continue
		}
		if byName[r.Name] == nil {
			byName[r.Name] = map[activity.Date]*activity.DaySlots{}
		}
		byName[r.Name][activity.Date(r.Date)] = &s
	}
	b.mu.Lock()
	var carried []ports.ActivityDay
	seed := func(l *activity.Ledger, days map[activity.Date]*activity.DaySlots) {
		for d, s := range days {
			l.Seed(d, *s)
		}
	}
	for name, in := range b.inboxes {
		if _, known := b.records[name]; !known {
			continue
		}
		if days, has := byName[name]; has {
			in.act = activity.FromDays(days, now, in.totals())
			seed(&in.days, days)
			continue
		}
		// The queue's own ring, from before durable days.
		days := in.act.Days(in.totals())
		seed(&in.days, days)
		carried = append(carried, dayRows(name, &in.days, activity.Closed{})...)
	}
	for name, days := range byName {
		if _, known := b.records[name]; !known || name == nodeSeries {
			continue
		}
		if _, has := b.inboxes[name]; !has {
			in := b.ensure(name)
			in.act = activity.FromDays(days, now, in.totals())
			seed(&in.days, days)
		}
	}
	if days, has := byName[nodeSeries]; has {
		b.node = activity.FromDays(days, now, b.refusedTotal())
		seed(&b.nodeDays, days)
	} else {
		seed(&b.nodeDays, b.node.Days(b.refusedTotal()))
		carried = append(carried, dayRows(nodeSeries, &b.nodeDays, activity.Closed{})...)
	}
	b.unlock()
	if len(carried) > 0 {
		if err := as.SaveActivityDays(carried); err != nil {
			b.report(ports.Error, "activity carried from before durable days not saved: %v", err)
		}
	}
}

func nameOr(n string) string {
	if n == nodeSeries {
		return "the node"
	}
	return n
}

// rangeDays is each wanted name's days in a range: one name, or every live
// record the caller may see, plus the node's refusals for the daemon Owner's
// unfiltered view. The days the rings hold come from them, older days from
// the store; the ring is the truth for what it covers, being newer than any
// row, and its open slot was never written.
func (b *Bus) rangeDays(caller, name string, from, to activity.Date) (map[string]map[activity.Date]activity.DaySlots, bool, error) {
	if !from.Valid() || !to.Valid() || to < from || from.AddDays(RangeDays-1) < to {
		return nil, false, ErrRange
	}
	if name != "" {
		var err error
		if name, err = canon(name); err != nil {
			return nil, false, err
		}
	}
	type live struct {
		ring  activity.Ring
		total Counts
		has   bool
	}
	b.mu.Lock()
	now := b.clock()
	if name != "" {
		r, known := b.entity(name)
		if !known || !b.canSee(caller, r) {
			b.unlock()
			return nil, false, ErrUnknown
		}
	}
	rings := map[string]live{}
	take := func(n string) {
		if in := b.inboxes[n]; in != nil {
			in.act.Tick(now, in.totals())
			rings[n] = live{in.act, in.totals(), true}
		} else {
			rings[n] = live{}
		}
	}
	if name != "" {
		take(name)
	} else {
		for n, r := range b.records {
			if b.live(r) && b.canSee(caller, r) {
				take(n)
			}
		}
	}
	nodeWide := name == "" && caller == b.admin
	if nodeWide {
		b.node.Tick(now, b.refusedTotal())
		rings[nodeSeries] = live{b.node, b.refusedTotal(), true}
	}
	as := b.activityStore()
	b.unlock()

	names := make([]string, 0, len(rings))
	for n := range rings {
		names = append(names, n)
	}
	sort.Strings(names)
	stored := map[string]map[activity.Date]*activity.DaySlots{}
	if as != nil {
		rows, err := as.ActivityDays(int(from), int(to), names)
		if err != nil {
			return nil, false, err
		}
		for _, r := range rows {
			if _, wanted := rings[r.Name]; !wanted {
				continue
			}
			s, err := activity.DecodeDay(r.Slots)
			if err != nil {
				continue
			}
			if stored[r.Name] == nil {
				stored[r.Name] = map[activity.Date]*activity.DaySlots{}
			}
			stored[r.Name][activity.Date(r.Date)] = &s
		}
	}
	out := map[string]map[activity.Date]activity.DaySlots{}
	for n, l := range rings {
		days := map[activity.Date]activity.DaySlots{}
		for d := from; d <= to; d = d.AddDays(1) {
			day := activity.DaySlots{}
			if s := stored[n][d]; s != nil {
				day = *s
			}
			if l.has {
				ringDay := l.ring.Day(d, l.total)
				for i := range day {
					if l.ring.Covers(d, i) {
						day[i] = ringDay[i]
					}
				}
			}
			days[d] = day
		}
		out[n] = days
	}
	return out, nodeWide, nil
}

// ActivityDays is a range of calendar days of one name, or the sum of every
// live record the caller may see: one entry per day, from first to last. On
// the sum the daemon Owner's Refused is node-wide, as Activity's is.
func (b *Bus) ActivityDays(caller, name string, from, to activity.Date) ([]ActivityDay, error) {
	per, nodeWide, err := b.rangeDays(caller, name, from, to)
	if err != nil {
		return nil, err
	}
	loc := b.Now().Location()
	var out []ActivityDay
	for d := from; d <= to; d = d.AddDays(1) {
		var sum activity.DaySlots
		for n, days := range per {
			if n == nodeSeries {
				continue
			}
			day := days[d]
			sum.Add(&day)
		}
		if nodeWide {
			node := per[nodeSeries][d]
			for i := range sum {
				sum[i].Refused = node[i].Refused
			}
		}
		out = append(out, ActivityDay{Date: d, Slots: activity.SlotsOf(d, &sum, loc)})
	}
	return out, nil
}

// ActivityTotals is each visible record's counts summed over a range, in one
// answer: what a chooser needs without a range per record.
func (b *Bus) ActivityTotals(caller string, from, to activity.Date) (map[string]Counts, error) {
	per, _, err := b.rangeDays(caller, "", from, to)
	if err != nil {
		return nil, err
	}
	out := map[string]Counts{}
	for n, days := range per {
		if n == nodeSeries {
			continue
		}
		var c Counts
		for _, day := range days {
			for _, s := range day {
				c.In, c.Out, c.Dropped, c.Expired, c.Refused = c.In+s.In, c.Out+s.Out, c.Dropped+s.Dropped, c.Expired+s.Expired, c.Refused+s.Refused
			}
		}
		out[n] = c
	}
	return out, nil
}

// Today is the node's calendar day, as the activity days key it.
func (b *Bus) Today() activity.Date { return activity.DateOf(b.Now()) }

// Now is the time as the bus reads it.
func (b *Bus) Now() time.Time {
	b.mu.Lock()
	defer b.unlock()
	return b.clock()
}
