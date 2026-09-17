package core

import "time"

// One hour of minute samples plus the baseline needed for the first delta.
const activityKept = 61

type Counts struct {
	In      int `json:"in"`
	Out     int `json:"out"`
	Dropped int `json:"dropped"`
	Expired int `json:"expired"`
	Refused int `json:"refused"`
}
type ActivityPoint struct {
	At time.Time `json:"at"`
	Counts
}
type activitySample struct {
	at      time.Time
	records map[string]Counts
	refused int
}

func (b *Bus) activitySnapshot(now time.Time) activitySample {
	s := activitySample{at: now, records: map[string]Counts{}}
	for name := range b.records {
		if in := b.inboxes[name]; in != nil {
			s.records[name] = Counts{in.in, in.out, in.dropped, in.expired, in.refused}
		}
	}
	for _, n := range b.refused {
		s.refused += n
	}
	return s
}

// SampleActivity is driven by the bus process, independently of page visits.
func (b *Bus) SampleActivity(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.activity) > 0 && !now.After(b.activity[len(b.activity)-1].at) {
		return
	}
	if len(b.activity) == activityKept {
		copy(b.activity, b.activity[1:])
		b.activity = b.activity[:activityKept-1]
	}
	b.activity = append(b.activity, b.activitySnapshot(now))
}

func (b *Bus) RecordRefusal(name string) {
	name, err := canon(name)
	if err != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if in := b.inboxes[name]; in != nil {
		in.refused++
	}
}

func (b *Bus) Activity(caller, name string) ([]ActivityPoint, error) {
	if name != "" {
		var err error
		name, err = canon(name)
		if err != nil {
			return nil, err
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if name != "" {
		r, known := b.records[name]
		if !known || !b.may(caller, r) {
			return nil, ErrUnknown
		}
	}
	samples := append([]activitySample{}, b.activity...)
	samples = append(samples, b.activitySnapshot(time.Now()))
	points := []ActivityPoint{}
	total := func(s activitySample) Counts {
		c := Counts{}
		for n, r := range b.records {
			if name != "" && n != name || !b.may(caller, r) {
				continue
			}
			v := s.records[n]
			c.In += v.In
			c.Out += v.Out
			c.Dropped += v.Dropped
			c.Expired += v.Expired
			c.Refused += v.Refused
		}
		if name == "" && (caller == b.admin || b.masters[caller]) {
			c.Refused = s.refused
		}
		return c
	}
	for i := 1; i < len(samples); i++ {
		if time.Since(samples[i].at) > time.Hour {
			continue
		}
		prev, next := total(samples[i-1]), total(samples[i])
		points = append(points, ActivityPoint{samples[i].at, Counts{max(0, next.In-prev.In), max(0, next.Out-prev.Out), max(0, next.Dropped-prev.Dropped), max(0, next.Expired-prev.Expired), max(0, next.Refused-prev.Refused)}})
	}
	return points, nil
}
