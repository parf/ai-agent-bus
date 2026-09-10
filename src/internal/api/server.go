// Package api is a face: it turns an HTTP request into a core call and does
// nothing else. No domain logic lives here. See docs/10-modules.md.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The two parameters every call carries, on the wire.
// See docs/02-access.md#two-parameters.
const (
	HeaderUser  = "X-Agent-Bus-User"
	HeaderToken = "X-Agent-Bus-Token"
)

const maxWait = 60 * time.Second

type Server struct {
	bus   *core.Bus
	token string
}

func New(bus *core.Bus, token string) *Server { return &Server{bus: bus, token: token} }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", s.auth(s.status))
	mux.HandleFunc("POST /register", s.auth(s.register))
	mux.HandleFunc("GET /ls", s.auth(s.ls))
	mux.HandleFunc("POST /send", s.auth(s.send))
	mux.HandleFunc("GET /consume", s.auth(s.consume))
	return mux
}

// auth checks the two parameters. In PoC one master token per user reaches
// every service, so this is a string compare and no ACL at all.
func (s *Server) auth(next func(http.ResponseWriter, *http.Request, protocol.Name)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, err := protocol.ParseName(r.Header.Get(HeaderUser))
		if err != nil {
			fail(w, http.StatusUnauthorized, "bad or missing "+HeaderUser)
			return
		}
		if r.Header.Get(HeaderToken) != s.token {
			fail(w, http.StatusUnauthorized, "bad token")
			return
		}
		next(w, r, name)
	}
}

func (s *Server) status(w http.ResponseWriter, r *http.Request, _ protocol.Name) {
	ok(w, s.bus.Status())
}

func (s *Server) register(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in protocol.Record
	if !read(w, r, &in) {
		return
	}
	in.Owner = caller.String()
	rec, err := s.bus.Register(in)
	if errors.Is(err, core.ErrBadName) {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, rec)
}

func (s *Server) ls(w http.ResponseWriter, r *http.Request, _ protocol.Name) {
	ok(w, s.bus.List(r.URL.Query().Get("kind")))
}

func (s *Server) send(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in protocol.Envelope
	if !read(w, r, &in) {
		return
	}
	in.From = caller.String()
	e, err := s.bus.Send(in)
	switch {
	case errors.Is(err, core.ErrBadName):
		fail(w, http.StatusBadRequest, err.Error())
		return
	case errors.Is(err, core.ErrUnknown):
		fail(w, http.StatusNotFound, "no such receiver: "+in.To)
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, e)
}

// consume long-polls the caller's own inbox. topic+tag make it a filtered
// wait, which the daemon serves ahead of the unfiltered reader.
func (s *Server) consume(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	q := r.URL.Query()
	topic, tag := q.Get("topic"), q.Get("tag")
	filtered := topic != "" || tag != ""

	wait := 30 * time.Second
	if v := q.Get("wait"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			wait = min(d, maxWait)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), wait)
	defer cancel()

	e, err := s.bus.Consume(ctx, caller.String(), topic, tag, filtered)
	switch {
	case errors.Is(err, core.ErrTwoReads):
		fail(w, http.StatusConflict, "this inbox already has a reader")
	case err != nil:
		w.WriteHeader(http.StatusNoContent) // nothing arrived in time
	default:
		ok(w, e)
	}
}

func read(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		fail(w, http.StatusBadRequest, "bad json: "+err.Error())
		return false
	}
	return true
}

func ok(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
