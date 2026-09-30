// The locks API: named locks in a Group, one holder at a time, a TTL on every
// one, memory only. The Group is the namespace and the ACL — membership and
// liveness come from the registry, the table from the locks package, and this
// file is the wiring between them
// (docs/01-identity-and-roles.md#shared-locks).
package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/locks"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

const maxLockWait = 60 * time.Second

// held is the codes-table sentinel for any HeldBy: a lock somebody else holds
// is the one refusal that is neither authority nor shape, and the dashboard
// counts it apart (docs/05-discovery.md#refusals).
var held = &locks.HeldBy{}

// selfTake is the codes-table sentinel for a holder re-taking their own lock.
var selfTake = &locks.SelfTake{}

func (s *Server) lockArgs(w http.ResponseWriter, r *http.Request) (group, name string, ttl time.Duration, ok bool) {
	var in struct {
		Group string `json:"group"`
		Name  string `json:"name"`
		TTL   string `json:"ttl"`
	}
	if !s.read(w, r, &in) {
		return "", "", 0, false
	}
	d, err := time.ParseDuration(in.TTL)
	if err != nil || d <= 0 || d > 24*time.Hour {
		s.reply(w, nil, core.ErrTTL)
		return "", "", 0, false
	}
	return in.Group, in.Name, d, true
}

// gateLock answers the two registry questions every lock call asks: is the
// group there and active (an inactive group has no locks, and the ones it
// held are gone), and is the caller a member. Refusals are the registry's own
// words: unknown and inactive read the same, as they do everywhere.
func (s *Server) gateLock(w http.ResponseWriter, caller protocol.Name, group string) bool {
	if !s.bus.GroupLive(group) {
		s.reply(w, nil, core.ErrUnknown)
		return false
	}
	if !s.bus.InGroup(caller.String(), group) {
		s.reply(w, nil, core.ErrNotAllow)
		return false
	}
	return true
}

func (s *Server) lockTake(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	group, name, ttl, ok := s.lockArgs(w, r)
	if !ok {
		return
	}
	var wait time.Duration
	if v := r.URL.Query().Get("wait"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < 0 || d > maxLockWait {
			s.reply(w, nil, core.ErrWait)
			return
		}
		wait = d
	}
	if r.URL.Path == "/try-lock" {
		wait = 0
	}
	if !s.gateLock(w, caller, group) {
		return
	}
	granted, who := s.locks.Take(r.Context(), group, name, caller.String(), ttl, wait)
	if !granted {
		if who == caller.String() {
			s.reply(w, lockAnswer{Group: group, Name: name, Holder: who}, selfTake)
			return
		}
		s.reply(w, lockAnswer{Group: group, Name: name, Holder: who}, &locks.HeldBy{Holder: who})
		return
	}
	s.reply(w, lockAnswer{Group: group, Name: name, Holder: caller.String()}, nil)
}

func (s *Server) lockRelease(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Group string `json:"group"`
		Name  string `json:"name"`
	}
	if !s.read(w, r, &in) {
		return
	}
	group, name := in.Group, in.Name
	if !s.gateLock(w, caller, group) {
		return
	}
	err := s.locks.Release(group, name, caller.String(), false)
	var by *locks.HeldBy
	if errors.As(err, &by) {
		s.reply(w, lockAnswer{Group: group, Name: name, Holder: by.Holder}, err)
		return
	}
	s.reply(w, lockAnswer{Group: group, Name: name}, err)
}

func (s *Server) lockExtend(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	group, name, ttl, ok := s.lockArgs(w, r)
	if !ok {
		return
	}
	if !s.gateLock(w, caller, group) {
		return
	}
	err := s.locks.Extend(group, name, caller.String(), ttl)
	var by *locks.HeldBy
	if errors.As(err, &by) {
		s.reply(w, lockAnswer{Group: group, Name: name, Holder: by.Holder}, err)
		return
	}
	s.reply(w, lockAnswer{Group: group, Name: name, Holder: caller.String()}, err)
}

func (s *Server) lockHolders(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	group := r.URL.Query().Get("group")
	if !s.gateLock(w, caller, group) {
		return
	}
	s.reply(w, lockHolders{Group: group, Locks: s.locks.Holders(group)}, nil)
}

type lockAnswer struct {
	Group  string `json:"group"`
	Name   string `json:"name,omitempty"`
	Holder string `json:"holder,omitempty"`
}

type lockHolders struct {
	Group string            `json:"group"`
	Locks map[string]string `json:"locks"`
}

// lockReleaseForce is --force as its own operation: any member may release a
// lock somebody else holds, and the audit says who displaced whom. The plain
// release stays unaudited, as any ordinary act of one's own does.
func (s *Server) lockReleaseForce(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Group string `json:"group"`
		Name  string `json:"name"`
	}
	if !s.read(w, r, &in) {
		return
	}
	if !s.gateLock(w, caller, in.Group) {
		return
	}
	err := s.locks.Release(in.Group, in.Name, caller.String(), true)
	var displaced *locks.Displaced
	if errors.As(err, &displaced) {
		s.reply(w, lockAnswer{Group: in.Group, Name: in.Name, Holder: displaced.Previous}, nil)
		return
	}
	s.reply(w, lockAnswer{Group: in.Group, Name: in.Name}, err)
}
