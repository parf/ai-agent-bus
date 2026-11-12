package memory

// Each record's schedules and outcomes, as the database keeps them
// (Plans/R1.0-Release/cron-in-core.md#storage).

import (
	"sort"

	"github.com/parf/ai-agent-bus/internal/ports"
)

// dropSchedules removes every schedule and outcome of one record. Caller holds s.mu.
func (s *State) dropSchedules(record uint32) {
	for id, v := range s.schedules {
		if v.Record == record {
			delete(s.schedules, id)
		}
	}
	delete(s.outcomes, record)
}

func (s *State) ScheduleGet(id string) (ports.Schedule, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.schedules[id]
	return v, ok, nil
}

func (s *State) ScheduleEdit(id string, record uint32, edit func(ports.Schedule, bool, ports.ScheduleCount) (ports.ScheduleChange, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.holds(record) {
		return ports.ErrKVNoRecord
	}
	old, found := s.schedules[id]
	if found && old.Record != record {
		old, found = ports.Schedule{}, false
	}
	var count ports.ScheduleCount
	for _, v := range s.schedules {
		if v.Record != record {
			continue
		}
		if v.Once {
			count.Once++
		} else {
			count.Recurring++
		}
	}
	c, err := edit(old, found, count)
	if err != nil {
		return err
	}
	switch {
	case c.Delete:
		delete(s.schedules, id)
	case c.Write:
		v := c.Value
		v.ID, v.Record = id, record
		s.schedules[id] = v
	}
	if o := c.Outcome; o != nil {
		o.Schedule, o.Record = id, record
		kept := append([]ports.ScheduleOutcome{*o}, s.outcomes[record]...)
		fresh := kept[:0]
		for _, k := range kept {
			if k.At >= o.At-ports.ScheduleOutcomeAge && len(fresh) < ports.ScheduleOutcomesKept {
				fresh = append(fresh, k)
			}
		}
		s.outcomes[record] = append([]ports.ScheduleOutcome(nil), fresh...)
	}
	return nil
}

func (s *State) pick(keep func(ports.Schedule) bool) []ports.Schedule {
	out := []ports.Schedule{}
	for _, v := range s.schedules {
		if keep(v) {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *State) ScheduleList(record uint32) ([]ports.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pick(func(v ports.Schedule) bool { return v.Record == record }), nil
}

func (s *State) ScheduleDue(now int64, limit int) ([]ports.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.pick(func(v ports.Schedule) bool { return v.Enabled && v.Next > 0 && v.Next <= now })
	sort.SliceStable(out, func(i, j int) bool { return out[i].Next < out[j].Next })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *State) ScheduleNext() (int64, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var next int64
	for _, v := range s.schedules {
		if v.Enabled && v.Next > 0 && (next == 0 || v.Next < next) {
			next = v.Next
		}
	}
	return next, next > 0, nil
}

func (s *State) ScheduleInFlight() ([]ports.Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pick(func(v ports.Schedule) bool { return v.InFlight > 0 }), nil
}

func (s *State) ScheduleCounts() (map[uint32]ports.ScheduleCount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[uint32]ports.ScheduleCount{}
	for _, v := range s.schedules {
		c := out[v.Record]
		if v.Once {
			c.Once++
		} else {
			c.Recurring++
		}
		out[v.Record] = c
	}
	return out, nil
}

func (s *State) ScheduleOutcomes(record uint32, schedule string) ([]ports.ScheduleOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []ports.ScheduleOutcome{}
	for _, o := range s.outcomes[record] {
		if schedule == "" || o.Schedule == schedule {
			out = append(out, o)
		}
	}
	return out, nil
}

func (s *State) ScheduleOrphans() ([]uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[uint32]bool{}
	var out []uint32
	for _, v := range s.schedules {
		if !seen[v.Record] && !s.holds(v.Record) {
			seen[v.Record] = true
			out = append(out, v.Record)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// ScheduleBroken is always empty: the memory store holds decoded values only.
func (s *State) ScheduleBroken() ([]string, error) { return nil, nil }
