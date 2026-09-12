// Package api is a face: it turns an HTTP request into a core call and does
// nothing else. No domain logic lives here. See docs/10-modules.md.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Dial is how every client reaches the daemon: HTTP over a unix socket or
// over loopback, the same protocol on both. One place, because the CLI and
// the dashboard are two processes that must agree about it.
// See docs/02-access.md#local-socket.
func Dial(addr string) (*http.Client, string) {
	if addr == "" {
		addr = DefaultSocket()
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	if strings.HasPrefix(addr, "http://") {
		return client, strings.TrimSuffix(addr, "/")
	}
	client.Transport = &http.Transport{
		IdleConnTimeout: 90 * time.Second,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", addr)
		},
	}
	return client, "http://unix"
}

// The two parameters every call carries, on the wire.
// See docs/02-access.md#two-parameters.
const (
	HeaderUser  = "X-Agent-Bus-User"
	HeaderToken = "X-Agent-Bus-Token"
)

// SystemRuntimeDir is where a daemon installed for the whole host keeps its
// sockets: outside anyone's home, and cleared by a reboot.
// See docs/02-access.md#local-socket.
const SystemRuntimeDir = "/run/agent-bus"

// SystemSocket is that daemon's shared listener, beside the per-account ones.
func SystemSocket() string { return filepath.Join(SystemRuntimeDir, "bus.sock") }

// DefaultSocket is where the daemon listens and the CLI looks, in one place
// because the two binaries have to agree — see docs/02-access.md#local-socket.
func DefaultSocket() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "agent-bus", "bus.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("agent-bus-%d.sock", os.Getuid()))
}

// UserSocket is one account's own socket in the daemon's socket directory.
// The name carries the account so that a person, a script and the daemon all
// work it out the same way. See docs/02-access.md#local-socket.
func UserSocket(dir, account string) string {
	return filepath.Join(dir, "user-"+account+".sock")
}

// IsUserSocket says whether a path is one of those, which is how a client
// knows the two parameters will be supplied for it.
func IsUserSocket(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, "user-") && strings.HasSuffix(base, ".sock")
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

// guard turns a handler that needs a caller into one that does not, by
// working out who the caller is. There are two: the credential on the
// request, and the socket it arrived on.
type guard func(func(http.ResponseWriter, *http.Request, protocol.Name)) http.HandlerFunc

// Handler serves the listeners anyone can reach, where a request carries its
// own two parameters.
func (s *Server) Handler() http.Handler { return s.routes(s.auth) }

// HandlerFor serves one account's own socket. The socket supplies the two
// parameters instead of the caller — it does not replace them, so a request
// that states a *different* name is refused exactly as it would be from
// anywhere else. See docs/02-access.md#local-socket.
func (s *Server) HandlerFor(principal protocol.Name) http.Handler {
	return s.routes(s.onSocket(principal))
}

func (s *Server) routes(g guard) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", g(s.status))
	mux.HandleFunc("POST /register", g(s.register))
	mux.HandleFunc("GET /ls", g(s.ls))
	mux.HandleFunc("GET /lookup", g(s.lookup))
	mux.HandleFunc("GET /recent", g(s.recent))
	mux.HandleFunc("POST /subscribe", g(s.subscribe))
	mux.HandleFunc("POST /configure", g(s.configure))
	mux.HandleFunc("GET /config", g(s.config))
	mux.HandleFunc("POST /send", g(s.send))
	mux.HandleFunc("GET /consume", g(s.consume))
	mux.HandleFunc("POST /token", g(s.token))
	// Enrolment is the one route with no credential on it, because it is
	// where a credential comes from — requiring one would be a circle. It is
	// safe for the same reason: the signature *is* the credential, and only a
	// realm somebody vouches for can be enrolled into at all.
	// See docs/01-identity.md#proving-possession.
	mux.HandleFunc("POST /enrol", func(w http.ResponseWriter, r *http.Request) {
		s.enrol(w, r, protocol.Name{})
	})
	return mux
}

// auth checks the two parameters, and checks them against each other: the
// name is bound to the credential it arrived with, so a caller cannot be
// somebody else. See docs/02-access.md#two-parameters.
func (s *Server) auth(next func(http.ResponseWriter, *http.Request, protocol.Name)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, err := protocol.ParseName(r.Header.Get(HeaderUser))
		if err != nil {
			s.bus.Refuse("credential")
			fail(w, http.StatusUnauthorized, "bad or missing "+HeaderUser)
			return
		}
		who, known := s.tokens.Principal(r.Header.Get(HeaderToken))
		if !known {
			s.bus.Refuse("credential")
			fail(w, http.StatusUnauthorized, "bad token")
			return
		}
		// Authenticated, and asking to be read as someone else. That is a
		// different answer from "no token": saying so is what makes the
		// refusal debuggable instead of looking like a bad credential.
		if who != name.String() {
			s.bus.Refuse("wrong-name")
			fail(w, http.StatusForbidden, "that token belongs to "+who+", not "+name.String())
			return
		}
		next(w, r, name)
	}
}

// onSocket is the local path: the account at the other end is known from
// which socket the connection arrived on, so nothing has to be sent and
// there is nothing to set up. A name may still be stated — and had better
// be the right one.
func (s *Server) onSocket(me protocol.Name) guard {
	return func(next func(http.ResponseWriter, *http.Request, protocol.Name)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if stated := r.Header.Get(HeaderUser); stated != "" {
				name, err := protocol.ParseName(stated)
				if err != nil {
					s.bus.Refuse("credential")
					fail(w, http.StatusUnauthorized, "bad "+HeaderUser)
					return
				}
				if name.String() != me.String() {
					s.bus.Refuse("wrong-name")
					fail(w, http.StatusForbidden, "this socket is "+me.String()+"'s, not "+name.String())
					return
				}
			}
			next(w, r, me)
		}
	}
}

// token hands out a principal's credential, or rotates it. The daemon's
// owner may ask for any name; anyone else only for a name they already own,
// which is how a runner gets the credential for a script service it started.
// See docs/02-access.md#getting-a-token.
func (s *Server) token(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Name   string `json:"name"`
		Rotate bool   `json:"rotate,omitempty"`
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
		owner, known := s.bus.OwnerOf(want.String())
		if !known || owner != caller.String() {
			fail(w, http.StatusForbidden, caller.String()+" does not own "+want.String())
			return
		}
	}
	issue := s.tokens.Issue
	if in.Rotate {
		issue = s.tokens.Rotate
	}
	tok, err := issue(want.String())
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]string{"name": want.String(), "token": tok})
}

// status also answers "who am I" — the one question a caller on its own
// socket cannot answer for itself, because it never stated a name.
// See docs/02-access.md#local-socket.
func (s *Server) status(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	ok(w, struct {
		core.Status
		You string `json:"you"`
	}{s.bus.Status(), caller.String()})
}

// subscribe puts the caller on a pub/sub topic, or takes it off. The caller
// is the subscriber — there is no third name here, because a subscription
// puts messages in somebody's inbox and only they may ask for that.
// See docs/04-messaging.md#push-and-pull.
func (s *Server) subscribe(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Topic string `json:"topic"`
		Off   bool   `json:"off,omitempty"`
	}
	if !read(w, r, &in) {
		return
	}
	rec, err := s.bus.Subscribe(caller.String(), in.Topic, !in.Off)
	s.reply(w, rec, err)
}

func (s *Server) register(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in protocol.Record
	if !read(w, r, &in) {
		return
	}
	in.Owner = caller.String()
	rec, err := s.bus.Register(in)
	s.reply(w, rec, err)
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
	s.reply(w, rec, err)
}

// config hands one back. A listing never carries a configuration, so this is
// the only route to it, and it is the owner's or the service's own.
func (s *Server) config(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	cfg, err := s.bus.Config(r.URL.Query().Get("name"), caller.String())
	if err != nil {
		s.reply(w, nil, err)
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
func (s *Server) lookup(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	name := r.URL.Query().Get("name")
	rec, known := s.bus.Lookup(caller.String(), name)
	if !known {
		fail(w, http.StatusNotFound, "no such name: "+name)
		return
	}
	ok(w, rec)
}

// enrol is both halves of it: with no signature it hands back the nonce to
// sign, with one it checks the answer. Two calls because the bus has to be
// the one that says what gets signed — a challenge the caller chose proves
// nothing. See docs/01-identity.md#registration.
func (s *Server) enrol(w http.ResponseWriter, r *http.Request, _ protocol.Name) {
	var in struct {
		Name      string `json:"name"`
		Nonce     string `json:"nonce"`
		Signature string `json:"signature"`
	}
	if !read(w, r, &in) {
		return
	}
	if in.Signature == "" {
		nonce, err := s.bus.Challenge(in.Name)
		if err != nil {
			s.reply(w, nil, err)
			return
		}
		ok(w, struct {
			Name      string `json:"name"`
			Nonce     string `json:"nonce"`
			Namespace string `json:"namespace"`
		}{in.Name, nonce, protocol.SigNamespace})
		return
	}
	rec, err := s.bus.Enrol(in.Nonce, in.Signature)
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	// The credential comes with it: being enrolled and being able to speak
	// as the name are the same thing, and the proof that was just checked is
	// what a token would otherwise be asked for.
	// See docs/02-access.md#getting-a-token.
	token, err := s.tokens.Issue(rec.Name)
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	ok(w, struct {
		protocol.Record
		Token string `json:"token"`
	}{rec, token})
}

// recent is who has been talking to whom, for the dashboard. Bodies never
// reach it — they are struck out where the ring is written, not here.
// Master holders only: it is a view of the node rather than of any one
// service, so the per-service layer has nothing to say about it.
// See docs/05-discovery.md#dashboard.
func (s *Server) recent(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	if !s.bus.IsMaster(caller.String()) {
		fail(w, http.StatusForbidden, "the feed of envelopes is the node's, and master's to read")
		return
	}
	ok(w, s.bus.Recent())
}

func (s *Server) ls(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	ok(w, s.bus.List(caller.String(), r.URL.Query().Get("kind")))
}

func (s *Server) send(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in protocol.Envelope
	if !read(w, r, &in) {
		return
	}
	in.From = caller.String()
	e, err := s.bus.Send(in)
	s.reply(w, e, err)
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
// `share` says this reader is one of a pool, which is the only thing that
// lets a second unfiltered read wait beside it
// (docs/04-messaging.md#one-reader-per-inbox).
//
// A tag is what makes the second case: a reply always carries one, and a
// topic never doubles as a name when a tag is present.
func (s *Server) consume(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	q := r.URL.Query()
	topic, tag := q.Get("topic"), q.Get("tag")
	filtered := topic != "" || tag != ""
	inbox := caller.String()
	if topic != "" && tag == "" {
		rec, known := s.bus.Lookup(caller.String(), topic)
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

	e, err := s.bus.Consume(ctx, inbox, topic, tag, filtered, q.Has("share"))
	// Only the deadline running out means "nothing arrived", and it is the
	// error itself that says so — not whether ctx happens to be expired,
	// which it always is once wait=0s. Every other refusal is a real answer
	// the caller has to hear: turning an unknown name into 204 told a
	// reader to keep polling an inbox that will never exist.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.reply(w, e, err)
}

// codes is the one place a refusal becomes a status. A handler that decides
// this for itself is a handler that will disagree with the next one, and the
// message belongs to whoever knew what went wrong — core, not here.
var codes = []struct {
	err  error
	code int
	// What `status` counts this as. The kinds are the ones the dashboard
	// shows (docs/05-discovery.md#what-it-shows); everything a caller simply
	// got wrong is one kind, because "you sent nonsense" is one answer
	// however many ways there are to send it.
	kind string
}{
	{core.ErrBadName, http.StatusBadRequest, "malformed"},
	{core.ErrOverflow, http.StatusBadRequest, "malformed"},
	{core.ErrMode, http.StatusBadRequest, "malformed"},
	{core.ErrConfig, http.StatusBadRequest, "malformed"},
	{core.ErrReceipt, http.StatusBadRequest, "malformed"},
	{core.ErrTTL, http.StatusBadRequest, "malformed"},
	{core.ErrWait, http.StatusBadRequest, "malformed"},
	{core.ErrBound, http.StatusBadRequest, "malformed"},
	{core.ErrNotOwner, http.StatusForbidden, "acl"},
	{core.ErrPrivate, http.StatusForbidden, "acl"},
	{core.ErrNotAllow, http.StatusForbidden, "acl"},
	{core.ErrEnrol, http.StatusForbidden, "enrolment"},
	{core.ErrUnknown, http.StatusNotFound, "unknown"},
	{core.ErrFull, http.StatusServiceUnavailable, "full"},
	{core.ErrTwoReads, http.StatusConflict, "second-reader"},
}

// reply answers with v, or with the status this error maps to. An error no
// row claims is ours, not the caller's, so it is a 500 — and is not counted
// as a refusal, because refusing a caller and failing them are different
// things to be told about.
func (s *Server) reply(w http.ResponseWriter, v any, err error) {
	if err == nil {
		ok(w, v)
		return
	}
	for _, c := range codes {
		if errors.Is(err, c.err) {
			s.bus.Refuse(c.kind)
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
