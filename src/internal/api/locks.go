// The locks API: named locks in a Group, one holder at a time, a TTL on every
// one, memory only. The Group is the namespace and the ACL — membership and
// liveness come from the registry, the table from the locks package, and this
// file is the wiring between them
// (docs/01-identity-and-roles.md#shared-locks).
package api

import (
	"net/http"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/locks"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

const maxLockWait = 60 * time.Second

// heldBy wraps the holder so the refusal mapping can name it a busy refusal;
// a lock somebody else holds is the one refusal that is not a refusal of
// authority or shape, and the dashboard counts it apart
// (docs/05-discovery.md#refusals).
func heldBy(holder string) error {
	return &locks.HeldBy{Holder: holder}
}

// held is the codes-table sentinel for any HeldBy.
var held = &locks.HeldBy{}

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
		s.locks.DropGroup(group)
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
	granted, who := s.locks.Take(group, name, caller.String(), ttl, wait)
	if !granted {
		s.reply(w, lockAnswer{Group: group, Name: name, Holder: who}, heldBy(who))
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
	force := r.URL.Query().Get("force") == "1"
	if !s.gateLock(w, caller, group) {
		return
	}
	err := s.locks.Release(group, name, caller.String(), force)
	if holder, held := locks.Reason(err); held {
		s.reply(w, lockAnswer{Group: group, Name: name, Holder: holder}, err)
		return
	}
	s.reply(w, lockAnswer{Group: group, Name: name}, err)
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
