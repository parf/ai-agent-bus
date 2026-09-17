package api

import (
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"net/http"
)

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
	var in struct{ Topic, Subscriber string }
	if s.read(w, r, &in) {
		rec, err := s.bus.RemoveSubscriber(caller.String(), in.Topic, in.Subscriber)
		s.reply(w, rec, err)
	}
}

func (s *Server) activity(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	points, err := s.bus.Activity(caller.String(), r.URL.Query().Get("name"))
	s.reply(w, points, err)
}

func (s *Server) users(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	ok(w, s.bus.Users(caller.String(), s.tokens.Names()))
}
func (s *Server) user(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		protocol.User
		Create bool `json:"create,omitempty"`
	}
	if s.read(w, r, &in) {
		user, err := s.bus.SetUser(caller.String(), in.User, in.Create)
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

func (s *Server) userState(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct{ Name, State string }
	if s.read(w, r, &in) {
		u, err := s.bus.SetUserState(caller.String(), in.Name, in.State)
		s.reply(w, u, err)
	}
}
