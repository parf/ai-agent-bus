// The schedule API: each record's crons and ats, kept and fired by the daemon
// (Plans/R1.0-Release/cron-in-core.md#calls).
package api

import (
	"net/http"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// scheduleAnswer is a schedule on the wire, its times RFC 3339 in UTC.
type scheduleAnswer struct {
	ID          string          `json:"id"`
	Holder      string          `json:"holder"`
	HolderKind  string          `json:"holder_kind"`
	Kind        string          `json:"kind"` // recurring or once
	Expr        string          `json:"expr,omitempty"`
	At          string          `json:"at,omitempty"`
	Target      string          `json:"target"`
	Body        string          `json:"body"`
	Creator     string          `json:"creator"`
	Enabled     bool            `json:"enabled"`
	Disabled    string          `json:"disabled,omitempty"`
	Until       string          `json:"until,omitempty"`
	MaxRuns     int             `json:"max_runs,omitempty"`
	MaxLate     string          `json:"max_late,omitempty"`
	Next        string          `json:"next,omitempty"`
	Last        string          `json:"last,omitempty"`
	Runs        int             `json:"runs"`
	Sent        int             `json:"sent"`
	Dropped     int             `json:"dropped"`
	Streak      int             `json:"refused_in_a_row,omitempty"`
	Created     string          `json:"created"`
	Changed     string          `json:"changed"`
	LastOutcome string          `json:"last_outcome,omitempty"`
	Upcoming    []string        `json:"upcoming,omitempty"`
	Outcomes    []outcomeAnswer `json:"outcomes,omitempty"`
}

type outcomeAnswer struct {
	At      string `json:"at"`
	Slot    string `json:"slot"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail,omitempty"`
}

func stamp(sec int64) string {
	if sec == 0 {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}

func toAnswer(v core.ScheduleView) scheduleAnswer {
	kind := "recurring"
	if v.Once {
		kind = "once"
	}
	a := scheduleAnswer{ID: v.ID, Holder: v.Holder, Kind: kind, Expr: v.Expr, At: stamp(v.At), Target: v.Target, Body: v.Body,
		Creator: v.Creator, Enabled: v.Enabled, Disabled: v.Disabled, Until: stamp(v.Until), MaxRuns: v.MaxRuns,
		Next: stamp(v.Next), Last: stamp(v.Last), Runs: v.Runs, Sent: v.Sent, Dropped: v.Dropped, Streak: v.Streak,
		Created: stamp(v.Created), Changed: stamp(v.Changed), HolderKind: v.HolderKind, LastOutcome: v.LastOutcome}
	for _, t := range v.Upcoming {
		a.Upcoming = append(a.Upcoming, stamp(t))
	}
	if v.MaxLate > 0 {
		a.MaxLate = (time.Duration(v.MaxLate) * time.Second).String()
	}
	for _, o := range v.Outcomes {
		a.Outcomes = append(a.Outcomes, outcomeOf(o))
	}
	return a
}

func outcomeOf(o ports.ScheduleOutcome) outcomeAnswer {
	return outcomeAnswer{At: stamp(o.At), Slot: stamp(o.Slot), Outcome: o.Outcome, Detail: o.Detail}
}

// scheduleIn is an add or an edit.
type scheduleIn struct {
	ID       string `json:"id"`
	Record   string `json:"record"`
	Expr     string `json:"expr"`
	In       string `json:"in"`
	At       string `json:"at"`
	Target   string `json:"target"`
	Body     string `json:"body"`
	Until    string `json:"until"`
	MaxRuns  int    `json:"max_runs"`
	MaxLate  string `json:"max_late"`
	Disabled bool   `json:"disabled"`
}

func (in scheduleIn) request() core.ScheduleRequest {
	return core.ScheduleRequest{Expr: in.Expr, In: in.In, At: in.At, Target: in.Target, Body: in.Body,
		Until: in.Until, MaxRuns: in.MaxRuns, MaxLate: in.MaxLate, Disabled: in.Disabled}
}

func (s *Server) scheduleGet(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	v, err := s.bus.ScheduleGet(caller.String(), r.URL.Query().Get("id"))
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	s.reply(w, toAnswer(v), nil)
}

func (s *Server) scheduleList(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	list, err := s.bus.ScheduleList(caller.String(), r.URL.Query().Get("record"))
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	out := make([]scheduleAnswer, 0, len(list))
	for _, v := range list {
		out = append(out, toAnswer(v))
	}
	s.reply(w, map[string]any{"schedules": out}, nil)
}

func (s *Server) scheduleAdd(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in scheduleIn
	if !s.readStrict(w, r, &in) {
		return
	}
	v, warn, err := s.bus.ScheduleAdd(caller.String(), in.Record, in.request())
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	s.reply(w, map[string]any{"schedule": toAnswer(v), "warning": warn}, nil)
}

func (s *Server) scheduleEdit(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in scheduleIn
	if !s.readStrict(w, r, &in) {
		return
	}
	v, err := s.bus.ScheduleEdit(caller.String(), in.ID, in.request())
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	s.reply(w, map[string]any{"schedule": toAnswer(v)}, nil)
}

type scheduleID struct {
	ID string `json:"id"`
}

func (s *Server) scheduleEnable(on bool) func(http.ResponseWriter, *http.Request, protocol.Name) {
	return func(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
		var in scheduleID
		if !s.readStrict(w, r, &in) {
			return
		}
		v, err := s.bus.ScheduleEnable(caller.String(), in.ID, on)
		if err != nil {
			s.reply(w, nil, err)
			return
		}
		s.reply(w, map[string]any{"schedule": toAnswer(v)}, nil)
	}
}

func (s *Server) scheduleRun(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in scheduleID
	if !s.readStrict(w, r, &in) {
		return
	}
	o, err := s.bus.ScheduleRun(caller.String(), in.ID)
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	s.reply(w, map[string]any{"outcome": outcomeOf(o)}, nil)
}

func (s *Server) scheduleDelete(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in scheduleID
	if !s.readStrict(w, r, &in) {
		return
	}
	err := s.bus.ScheduleDelete(caller.String(), in.ID)
	s.reply(w, map[string]any{"id": in.ID, "deleted": err == nil}, err)
}
