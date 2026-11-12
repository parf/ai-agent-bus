package bus

import (
	"context"
	"net/url"
)

// Schedules are a record's crons and one-shot sends, kept and fired by the
// daemon (docs/09-daemon-api.md#schedules). The daemon judges every field; a
// refusal is its *BusError.
type Schedules struct {
	bus    *Bus
	record string
}

// ScheduleSpec is what an add or an edit takes: one of Expr, In or At.
type ScheduleSpec struct {
	Expr     string `json:"expr,omitempty"`
	In       string `json:"in,omitempty"`
	At       string `json:"at,omitempty"`
	Target   string `json:"target"`
	Body     string `json:"body"`
	Until    string `json:"until,omitempty"`
	MaxRuns  int    `json:"max_runs,omitempty"`
	MaxLate  string `json:"max_late,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Schedule is one schedule as the daemon answers it; times are RFC 3339 in UTC.
type Schedule struct {
	ID            string            `json:"id"`
	Holder        string            `json:"holder"`
	Kind          string            `json:"kind"`
	Expr          string            `json:"expr,omitempty"`
	At            string            `json:"at,omitempty"`
	Target        string            `json:"target"`
	Body          string            `json:"body"`
	Creator       string            `json:"creator"`
	Enabled       bool              `json:"enabled"`
	Disabled      string            `json:"disabled,omitempty"`
	Until         string            `json:"until,omitempty"`
	MaxRuns       int               `json:"max_runs,omitempty"`
	MaxLate       string            `json:"max_late,omitempty"`
	Next          string            `json:"next,omitempty"`
	Last          string            `json:"last,omitempty"`
	Runs          int               `json:"runs"`
	Sent          int               `json:"sent"`
	Dropped       int               `json:"dropped"`
	RefusedInARow int               `json:"refused_in_a_row,omitempty"`
	Created       string            `json:"created"`
	Changed       string            `json:"changed"`
	Outcomes      []ScheduleOutcome `json:"outcomes,omitempty"`
}

// ScheduleOutcome is one firing's result.
type ScheduleOutcome struct {
	At      string `json:"at"`
	Slot    string `json:"slot"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail,omitempty"`
}

// Add keeps a new schedule on the record; warning is set near a cap.
func (s Schedules) Add(ctx context.Context, spec ScheduleSpec) (sched Schedule, warning string, err error) {
	var v struct {
		Schedule Schedule `json:"schedule"`
		Warning  string   `json:"warning"`
	}
	body := struct {
		Record string `json:"record"`
		ScheduleSpec
	}{s.record, spec}
	err = s.bus.json(ctx, "POST", "/schedule/add", nil, body, &v)
	return v.Schedule, v.Warning, err
}

// List answers the record's schedules.
func (s Schedules) List(ctx context.Context) ([]Schedule, error) {
	return s.bus.scheduleList(ctx, url.Values{"record": {s.record}})
}

// Schedules answers every schedule on a record the caller manages.
func (b *Bus) Schedules(ctx context.Context) ([]Schedule, error) {
	return b.scheduleList(ctx, nil)
}

func (b *Bus) scheduleList(ctx context.Context, q url.Values) ([]Schedule, error) {
	var v struct {
		Schedules []Schedule `json:"schedules"`
	}
	e := b.json(ctx, "GET", "/schedule/list", q, nil, &v)
	return v.Schedules, e
}

// Get answers one schedule with its outcomes, newest first.
func (s Schedules) Get(ctx context.Context, id string) (Schedule, error) {
	var v Schedule
	e := s.bus.json(ctx, "GET", "/schedule", url.Values{"id": {id}}, nil, &v)
	return v, e
}

// Edit replaces a schedule's time, target, body and limits; the caller becomes its creator.
func (s Schedules) Edit(ctx context.Context, id string, spec ScheduleSpec) (Schedule, error) {
	body := struct {
		ID string `json:"id"`
		ScheduleSpec
	}{id, spec}
	return s.change(ctx, "/schedule/edit", body)
}

// Enable lets a schedule fire again; the caller becomes its creator.
func (s Schedules) Enable(ctx context.Context, id string) (Schedule, error) {
	return s.change(ctx, "/schedule/enable", map[string]string{"id": id})
}

// Disable stops a schedule firing and keeps it.
func (s Schedules) Disable(ctx context.Context, id string) (Schedule, error) {
	return s.change(ctx, "/schedule/disable", map[string]string{"id": id})
}

func (s Schedules) change(ctx context.Context, path string, body any) (Schedule, error) {
	var v struct {
		Schedule Schedule `json:"schedule"`
	}
	e := s.bus.json(ctx, "POST", path, nil, body, &v)
	return v.Schedule, e
}

// Run fires a schedule once now; its next slot does not move. A lost answer
// is UncertainOutcome and is not retried: the firing may have been sent.
func (s Schedules) Run(ctx context.Context, id string) (ScheduleOutcome, error) {
	var v struct {
		Outcome ScheduleOutcome `json:"outcome"`
	}
	e := s.bus.json(ctx, "POST", "/schedule/run", nil, map[string]string{"id": id}, &v)
	return v.Outcome, e
}

// Delete removes a schedule; one that is not there is a 404 refusal.
func (s Schedules) Delete(ctx context.Context, id string) (bool, error) {
	var v struct {
		Deleted bool `json:"deleted"`
	}
	e := s.bus.json(ctx, "POST", "/schedule/delete", nil, map[string]string{"id": id}, &v)
	return v.Deleted, e
}
