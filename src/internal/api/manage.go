package api

import (
	"fmt"
	"strconv"

	"github.com/parf/ai-agent-bus/internal/activity"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"net/http"
)

func (s *Server) accounts(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	view, err := s.bus.Accounts(caller.String())
	s.reply(w, view, err)
}

func (s *Server) account(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var change struct {
		Account   string `json:"account"`
		Principal string `json:"principal,omitempty"`
		Remove    bool   `json:"remove,omitempty"`
	}
	if !s.readStrict(w, r, &change) {
		return
	}
	// Decide daemon authority before probing the host account database. An
	// ordinary caller must not use this endpoint to enumerate OS accounts.
	if _, err := s.bus.Accounts(caller.String()); err != nil {
		s.reply(w, nil, err)
		return
	}
	if change.Account == s.protectedAccount {
		s.reply(w, nil, fmt.Errorf("%w: the daemon account mapping is implicit", core.ErrNotOwner))
		return
	}
	if !change.Remove {
		if s.localAccount == nil {
			s.reply(w, nil, fmt.Errorf("%w: local account validation is unavailable", core.ErrBadName))
			return
		}
		if err := s.localAccount(change.Account); err != nil {
			s.reply(w, nil, fmt.Errorf("%w: local account %q: %v", core.ErrBadName, change.Account, err))
			return
		}
	}
	view, err := s.bus.SetAccount(caller.String(), change.Account, change.Principal, change.Remove)
	s.reply(w, view, err)
}

func (s *Server) manage(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var change core.Management
	if s.read(w, r, &change) {
		rec, err := s.bus.Manage(caller.String(), change)
		s.reply(w, rec, err)
	}
}

func (s *Server) owner(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct{ Name string }
	if s.read(w, r, &in) {
		user, err := s.bus.TransferDaemonOwner(caller.String(), in.Name)
		s.reply(w, user, err)
	}
}
func (s *Server) groups(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	ok(w, s.bus.Groups(caller.String()))
}
func (s *Server) group(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var change struct {
		Name    string
		Members []string
		// Still read, and still refused. Dropping the field would let a
		// removal request decode as a membership save with no members,
		// which is a different operation answered with success.
		Remove bool
	}
	if s.read(w, r, &change) {
		if change.Remove {
			s.reply(w, nil, core.ErrNoRemoval)
			return
		}
		s.reply(w, nil, s.bus.SetGroup(caller.String(), change.Name, change.Members))
	}
}

func (s *Server) removeSubscriber(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	// `channel` is the record a subscriber is taken off. An envelope's
	// `topic` is a message label and is a different thing entirely.
	// See docs/07-channels.md.
	var in struct{ Channel, Subscriber string }
	if s.read(w, r, &in) {
		rec, err := s.bus.RemoveSubscriber(caller.String(), in.Channel, in.Subscriber)
		s.reply(w, rec, err)
	}
}

func (s *Server) activity(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	day, err := s.bus.Activity(caller.String(), r.URL.Query().Get("name"))
	var out []protocol.ActivitySlot
	if err == nil {
		out = make([]protocol.ActivitySlot, len(day))
		for i, d := range day {
			out[i] = protocol.ActivitySlot{At: d.At, In: d.In, Out: d.Out, Dropped: d.Dropped, Expired: d.Expired, Refused: d.Refused}
		}
	}
	s.reply(w, out, err)
}

// activityDays is a range of calendar days, from and to as yymmdd, both
// included (docs/05-discovery.md#activity-history).
func (s *Server) activityDays(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	q := r.URL.Query()
	from, err1 := strconv.Atoi(q.Get("from"))
	to, err2 := strconv.Atoi(q.Get("to"))
	if err1 != nil || err2 != nil {
		s.reply(w, nil, core.ErrRange)
		return
	}
	// totals=1: each visible record's counts summed over the range, in one
	// answer, instead of a range read per record.
	if q.Get("totals") == "1" {
		totals, err := s.bus.ActivityTotals(caller.String(), activity.Date(from), activity.Date(to))
		out := map[string]protocol.ActivitySlot{}
		for n, c := range totals {
			out[n] = protocol.ActivitySlot{In: c.In, Out: c.Out, Dropped: c.Dropped, Expired: c.Expired, Refused: c.Refused}
		}
		s.reply(w, out, err)
		return
	}
	days, err := s.bus.ActivityDays(caller.String(), q.Get("name"), activity.Date(from), activity.Date(to))
	var out []protocol.ActivityDay
	for _, d := range days {
		day := protocol.ActivityDay{Day: int(d.Date), Slots: make([]protocol.ActivitySlot, len(d.Slots))}
		for i, x := range d.Slots {
			day.Slots[i] = protocol.ActivitySlot{At: x.At, In: x.In, Out: x.Out, Dropped: x.Dropped, Expired: x.Expired, Refused: x.Refused}
		}
		out = append(out, day)
	}
	s.reply(w, out, err)
}

func (s *Server) users(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	ok(w, s.bus.Users(caller.String(), s.tokens.Names()))
}
func (s *Server) user(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		protocol.User
		Create            bool `json:"create,omitempty"`
		ProfileDetailsSet bool `json:"profile_details_set,omitempty"`
	}
	if s.read(w, r, &in) {
		details := in.ProfileDetailsSet || in.GithubCompany != "" || in.GithubLocation != "" || in.GithubTwitterUsername != ""
		user, err := s.bus.SetUserWithProfileDetails(caller.String(), in.User, in.Create, details)
		s.reply(w, user, err)
	}
}

func (s *Server) profile(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Email string `json:"email"`
	}
	if s.readStrict(w, r, &in) {
		user, err := s.bus.EditOwnEmail(caller.String(), in.Email)
		s.reply(w, user, err)
	}
}

func (s *Server) githubRefresh(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Name string `json:"name"`
	}
	if s.readStrict(w, r, &in) {
		user, err := s.bus.RefreshGithub(caller.String(), in.Name)
		s.reply(w, user, err)
	}
}

func (s *Server) userState(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct{ Name, Status string }
	if s.read(w, r, &in) {
		u, err := s.bus.SetUserState(caller.String(), in.Name, in.Status)
		s.reply(w, u, err)
	}
}
