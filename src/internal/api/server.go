// Package api is a face: it turns an HTTP request into a core call and does
// nothing else. No domain logic lives here. See docs/10-modules.md.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// The two parameters every call carries, on the wire.
// See docs/02-access.md#two-parameters.
const (
	HeaderUser  = "X-Agent-Bus-User"
	HeaderToken = "X-Agent-Bus-Token"
)

// DefaultSocket is where the daemon listens and the CLI looks, in one place
// because the two binaries have to agree — see docs/02-access.md#local-socket.
func DefaultSocket() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "agent-bus", "bus.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("agent-bus-%d.sock", os.Getuid()))
}

const maxWait = 60 * time.Second

type Server struct {
	bus    *core.Bus
	tokens *auth.Tokens
	// The principal this daemon belongs to. It is the one that may hand out
	// a credential for a name nobody owns yet — everyone else is limited to
	// names they own. See docs/02-access.md#getting-a-token.
	owner string
}

func New(bus *core.Bus, tokens *auth.Tokens, owner string) *Server {
	return &Server{bus: bus, tokens: tokens, owner: owner}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", s.auth(s.status))
	mux.HandleFunc("POST /register", s.auth(s.register))
	mux.HandleFunc("GET /ls", s.auth(s.ls))
	mux.HandleFunc("GET /lookup", s.auth(s.lookup))
	mux.HandleFunc("POST /configure", s.auth(s.configure))
	mux.HandleFunc("GET /config", s.auth(s.config))
	mux.HandleFunc("POST /send", s.auth(s.send))
	mux.HandleFunc("GET /consume", s.auth(s.consume))
	mux.HandleFunc("POST /token", s.auth(s.token))
	return mux
}

// auth checks the two parameters, and checks them against each other: the
// name is bound to the credential it arrived with, so a caller cannot be
// somebody else. See docs/02-access.md#two-parameters.
func (s *Server) auth(next func(http.ResponseWriter, *http.Request, protocol.Name)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, err := protocol.ParseName(r.Header.Get(HeaderUser))
		if err != nil {
			fail(w, http.StatusUnauthorized, "bad or missing "+HeaderUser)
			return
		}
		who, known := s.tokens.Principal(r.Header.Get(HeaderToken))
		if !known {
			fail(w, http.StatusUnauthorized, "bad token")
			return
		}
		// Authenticated, and asking to be read as someone else. That is a
		// different answer from "no token": saying so is what makes the
		// refusal debuggable instead of looking like a bad credential.
		if who != name.String() {
			fail(w, http.StatusForbidden, "that token belongs to "+who+", not "+name.String())
			return
		}
		next(w, r, name)
	}
}

// token hands out a principal's credential. The daemon's owner may ask for
// any name; anyone else only for a name they already own, which is how a
// runner gets the credential for a script service it started.
// See docs/02-access.md#getting-a-token.
func (s *Server) token(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Name string `json:"name"`
	}
	if !read(w, r, &in) {
		return
	}
	want, err := protocol.ParseName(in.Name)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if caller.String() != s.owner && caller.String() != want.String() {
		rec, known := s.bus.Lookup(want.String())
		if !known || rec.Owner != caller.String() {
			fail(w, http.StatusForbidden, caller.String()+" does not own "+want.String())
			return
		}
	}
	tok, err := s.tokens.Issue(want.String())
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]string{"name": want.String(), "token": tok})
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
	reply(w, rec, err)
}

// configure stores a service's configuration. The body carries the name and
// the configuration itself, which stays opaque all the way down: it is only
// checked for being JSON. See docs/03-services-and-topics.md#configuring-a-template.
func (s *Server) configure(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
	}
	if !read(w, r, &in) {
		return
	}
	rec, err := s.bus.Configure(in.Name, caller.String(), in.Config)
	reply(w, rec, err)
}

// config hands one back. A listing never carries a configuration, so this is
// the only route to it, and it is the owner's or the service's own.
func (s *Server) config(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	cfg, err := s.bus.Config(r.URL.Query().Get("name"), caller.String())
	if err != nil {
		reply(w, nil, err)
		return
	}
	if len(cfg) == 0 {
		cfg = json.RawMessage("null")
	}
	w.Header().Set("Content-Type", "application/json")
	// cfg is the stored bytes; append() would write the newline into their
	// spare capacity, which two concurrent readers then race on.
	w.Write(cfg)
	w.Write([]byte{'\n'})
}

// lookup answers about one name. A caller asking "am I registered?" would
// otherwise pull the whole registry down to find out: List plus its JSON is
// milliseconds and megabytes at ten thousand records, while this is a map
// read. See docs/03-services-and-topics.md.
func (s *Server) lookup(w http.ResponseWriter, r *http.Request, _ protocol.Name) {
	name := r.URL.Query().Get("name")
	rec, known := s.bus.Lookup(name)
	if !known {
		fail(w, http.StatusNotFound, "no such name: "+name)
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
	reply(w, e, err)
}

// consume long-polls an inbox. Which one, and whether it is filtered, is
// decided here so that every face gets the same answer:
//
//   - `topic` alone, naming a **registered topic** → read that topic's inbox.
//     A queue topic is an inbox with a name (docs/12-stages.md#poc), and
//     there is one reader of it like any other inbox.
//   - otherwise `topic` and `tag` **filter the caller's own inbox** — the
//     wait a reply is collected on (docs/04-messaging.md#request-and-reply).
//
// A tag is what makes the second case: a reply always carries one, and a
// topic never doubles as a name when a tag is present.
func (s *Server) consume(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	q := r.URL.Query()
	topic, tag := q.Get("topic"), q.Get("tag")
	filtered := topic != "" || tag != ""
	inbox := caller.String()
	if topic != "" && tag == "" {
		rec, known := s.bus.Lookup(topic)
		switch {
		case known && rec.Kind == protocol.KindTopic:
			inbox, topic, filtered = rec.Name, "", false
		case strings.Contains(topic, "@"):
			// A topic filter is a label (`deploy-42`); a topic *name* is a
			// name (`jobs@srv1`). Saying the second and meaning the first is
			// a typo, and answering it with a silent timeout hides it.
			fail(w, http.StatusNotFound, "no such topic: "+topic)
			return
		}
	}

	wait := 30 * time.Second
	if v := q.Get("wait"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			wait = min(d, maxWait)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), wait)
	defer cancel()

	e, err := s.bus.Consume(ctx, inbox, topic, tag, filtered)
	// Only the deadline running out means "nothing arrived", and it is the
	// error itself that says so — not whether ctx happens to be expired,
	// which it always is once wait=0s. Every other refusal is a real answer
	// the caller has to hear: turning an unknown name into 204 told a
	// reader to keep polling an inbox that will never exist.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	reply(w, e, err)
}

// codes is the one place a refusal becomes a status. A handler that decides
// this for itself is a handler that will disagree with the next one, and the
// message belongs to whoever knew what went wrong — core, not here.
var codes = []struct {
	err  error
	code int
}{
	{core.ErrBadName, http.StatusBadRequest},
	{core.ErrOverflow, http.StatusBadRequest},
	{core.ErrMode, http.StatusBadRequest},
	{core.ErrConfig, http.StatusBadRequest},
	{core.ErrReceipt, http.StatusBadRequest},
	{core.ErrTTL, http.StatusBadRequest},
	{core.ErrBound, http.StatusBadRequest},
	{core.ErrNotOwner, http.StatusForbidden},
	{core.ErrPrivate, http.StatusForbidden},
	{core.ErrUnknown, http.StatusNotFound},
	{core.ErrFull, http.StatusServiceUnavailable},
	{core.ErrTwoReads, http.StatusConflict},
	{core.ErrNotYet, http.StatusNotImplemented},
}

// reply answers with v, or with the status this error maps to. An error no
// row claims is ours, not the caller's, so it is a 500.
func reply(w http.ResponseWriter, v any, err error) {
	if err == nil {
		ok(w, v)
		return
	}
	for _, c := range codes {
		if errors.Is(err, c.err) {
			fail(w, c.code, err.Error())
			return
		}
	}
	fail(w, http.StatusInternalServerError, err.Error())
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
