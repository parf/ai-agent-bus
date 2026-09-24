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
// for each ring whose just-closed slot counted anything, that slot's day,
// whole. Caller holds b.mu.
func (b *Bus) closing(now time.Time) []ports.ActivityDay {
	var rows []ports.ActivityDay
	for name, in := range b.inboxes {
		if d, write := in.act.TickClosing(now, in.totals()); write {
			if _, known := b.records[name]; known {
				s := in.act.Day(d, in.totals())
				rows = append(rows, ports.ActivityDay{Date: int(d), Name: name, Slots: activity.EncodeDay(&s)})
			}
		}
	}
	if d, write := b.node.TickClosing(now, b.refusedTotal()); write {
		s := b.node.Day(d, b.refusedTotal())
		rows = append(rows, ports.ActivityDay{Date: int(d), Name: nodeSeries, Slots: activity.EncodeDay(&s)})
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
		if err := as.PruneActivity(int(today.AddDays(-KeepDays))); err != nil {
			b.report(ports.Error, "old activity days not pruned: %v", err)
		}
	}
}

// FlushActivity writes every day the rings hold, the open slot's counts so far
// included: the graceful stop's, so a restart inside a slot keeps them.
func (b *Bus) FlushActivity() error {
	b.mu.Lock()
	as := b.activityStore()
	if as == nil {
		b.unlock()
		return nil
	}
	var rows []ports.ActivityDay
	add := func(name string, days map[activity.Date]*activity.DaySlots) {
		for d, s := range days {
			rows = append(rows, ports.ActivityDay{Date: int(d), Name: name, Slots: activity.EncodeDay(s)})
		}
	}
	for name, in := range b.inboxes {
		if _, known := b.records[name]; known {
			add(name, in.act.Days(in.totals()))
		}
	}
	add(nodeSeries, b.node.Days(b.refusedTotal()))
	b.unlock()
	if len(rows) == 0 {
		return nil
	}
	return as.SaveActivityDays(rows)
}

// RestoreActivityDays rebuilds each ring from the stored days of yesterday
// and today, after the store is bound. A name without rows keeps the ring its
// queue carried — how a database from before durable days starts — and a
// damaged row is reported and read as nothing.
func (b *Bus) RestoreActivityDays() {
	b.mu.Lock()
	as := b.activityStore()
	now := b.clock()
	b.unlock()
	if as == nil {
		return
	}
	today := activity.DateOf(now)
	rows, err := as.ActivityDays(int(today.AddDays(-1)), int(today))
	if err != nil {
		b.report(ports.Error, "activity days not read at start: %v", err)
		return
	}
	byName := map[string]map[activity.Date]*activity.DaySlots{}
	for _, r := range rows {
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
	defer b.unlock()
	for name, days := range byName {
		if name == nodeSeries {
			b.node = activity.FromDays(days, now, b.refusedTotal())
			continue
		}
		if _, known := b.records[name]; !known {
			continue
		}
		in := b.ensure(name)
		in.act = activity.FromDays(days, now, in.totals())
	}
}

func nameOr(n string) string {
	if n == nodeSeries {
		return "the node"
	}
	return n
}

// ActivityDays is a range of calendar days of one name, or the sum of every
// live record the caller may see: one entry per day, from first to last. The
// days the rings hold come from them, the rest from the store. On the sum the
// daemon Owner's Refused is node-wide, as Activity's is.
func (b *Bus) ActivityDays(caller, name string, from, to activity.Date) ([]ActivityDay, error) {
	if !from.Valid() || !to.Valid() || to < from || from.AddDays(RangeDays-1) < to {
		return nil, ErrRange
	}
	if name != "" {
		var err error
		if name, err = canon(name); err != nil {
			return nil, err
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
			return nil, ErrUnknown
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

	stored := map[string]map[activity.Date]*activity.DaySlots{}
	if as != nil {
		rows, err := as.ActivityDays(int(from), int(to))
		if err != nil {
			return nil, err
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
	var out []ActivityDay
	for d := from; d <= to; d = d.AddDays(1) {
		var sum, node activity.DaySlots
		for n, l := range rings {
			day := activity.DaySlots{}
			if s := stored[n][d]; s != nil {
				day = *s
			}
			// The ring is the truth for what it holds: its slots are newer
			// than any row, and the open one was never written.
			if l.has {
				ringDay := l.ring.Day(d, l.total)
				for i := range day {
					if l.ring.Covers(d, i) {
						day[i] = ringDay[i]
					}
				}
			}
			if n == nodeSeries {
				node = day
				continue
			}
			sum.Add(&day)
		}
		if nodeWide {
			for i := range sum {
				sum[i].Refused = node[i].Refused
			}
		}
		out = append(out, ActivityDay{Date: d, Slots: activity.SlotsOf(d, &sum, now.Location())})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}
