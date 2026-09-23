// Package api is a face: it turns an HTTP request into a core call and does
// nothing else. See docs/10-modules.md.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/ports"
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

// The one thing a call carries. There is no name beside it: the token backs
// exactly one principal, so a name on the wire could only agree or be a typo.
// See docs/02-access.md#what-a-call-carries.
const HeaderToken = "X-Agent-Bus-Token"

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

// ClientSocket discovers a local listener without changing the daemon's
// default bind path. A supplied token must use the shared listener.
func ClientSocket() string {
	dirs := []string{}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		dirs = append(dirs, filepath.Join(dir, "agent-bus"))
	}
	dirs = append(dirs, SystemRuntimeDir)
	account := ""
	if u, err := user.Current(); err == nil {
		account = u.Username
	}
	return clientSocket(dirs, account, os.Getenv("AGENT_BUS_TOKEN") != "")
}

func clientSocket(dirs []string, account string, hasToken bool) string {
	for _, dir := range dirs {
		path := filepath.Join(dir, "bus.sock")
		if !hasToken && account != "" {
			path = UserSocket(dir, account)
		}
		if fi, err := os.Stat(path); err == nil && fi.Mode()&os.ModeSocket != 0 {
			return path
		}
	}
	return DefaultSocket()
}

// UserSocket is one account's own socket in the daemon's socket directory.
// The name carries the account so that a person, a script and the daemon all
// work it out the same way. See docs/02-access.md#local-socket.
func UserSocket(dir, account string) string {
	return filepath.Join(dir, "user-"+account+".sock")
}

// IsUserSocket says whether a path is one of those, which is how a client
// knows the socket will supply its credential.
func IsUserSocket(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, "user-") && strings.HasSuffix(base, ".sock")
}

const maxWait = 60 * time.Second

type Server struct {
	journal          ports.Journal
	calls            func(time.Time) protocol.CallStats
	bus              *core.Bus
	tokens           *auth.Tokens
	localAccount     func(string) error
	protectedAccount string
	// Where a browser that arrived here is sent instead. Empty means the
	// root is not served at all, which is what it was before.
	dash string
}

// LocalAccounts supplies the host-only half of account-map validation. Core
// decides authority and principal standing; the assembled daemon decides
// whether an OS account exists. The daemon service account's implicit socket
// is deliberately outside the editable map.
func (s *Server) LocalAccounts(protected string, validate func(string) error) {
	s.protectedAccount, s.localAccount = protected, validate
}

// Dashboard says where a person who opened the API in a browser should have
// gone. The daemon cannot work it out: the dashboard is a separate process
// that picks its own port from whether it found a certificate
// (docs/05-discovery.md#where-it-listens), so it is told rather than guessed.
// Nowhere is a real answer, and it has to survive normalisation: an empty
// url trimmed and re-slashed would be "/", which is this root redirecting to
// itself for as long as the browser is willing.
func (s *Server) Dashboard(url string) {
	if url == "" {
		s.dash = ""
		return
	}
	s.dash = strings.TrimSuffix(url, "/") + "/"
}

func New(bus *core.Bus, tokens *auth.Tokens, owner string) *Server {
	if err := bus.EstablishDaemonOwner(owner); err != nil {
		panic(err)
	}
	// After the owner is established, so the owner's own credential binds
	// rather than being ignored as answering for nobody.
	bus.BindCredentials(tokens)
	return &Server{bus: bus, tokens: tokens}
}

// guard turns a handler that needs a caller into one that does not, by
// working out who the caller is. Two ways: the token on the request, and the
// socket it arrived on.
type guard func(func(http.ResponseWriter, *http.Request, protocol.Name)) http.HandlerFunc

// Handler serves the listeners anyone can reach, where a request carries its
// own token.
func (s *Server) Handler() http.Handler { return s.routes(s.auth) }

// HandlerFor serves one account's own socket. The socket is the credential:
// it stands in for the token, it is not an exemption from having one.
// See docs/02-access.md#local-socket.
func (s *Server) HandlerFor(principal protocol.Name) http.Handler {
	return s.routes(s.onSocket(principal))
}

func (s *Server) routes(g guard) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /identity", s.identity)
	mux.HandleFunc("GET /status", g(s.status))
	mux.HandleFunc("POST /register", g(s.audited("register", s.register)))
	mux.HandleFunc("POST /unregister", g(s.audited("unregister", s.unregister)))
	mux.HandleFunc("POST /manage", g(s.audited("manage", s.manage)))
	mux.HandleFunc("POST /owner", g(s.audited("transfer-daemon-owner", s.owner)))
	mux.HandleFunc("GET /accounts", g(s.accounts))
	mux.HandleFunc("POST /account", g(s.audited("account-map", s.account)))
	mux.HandleFunc("GET /groups", g(s.groups))
	mux.HandleFunc("GET /users", g(s.users))
	mux.HandleFunc("POST /user", g(s.audited("user", s.user)))
	mux.HandleFunc("POST /user/github-refresh", g(s.audited("github-refresh", s.githubRefresh)))
	mux.HandleFunc("POST /profile", g(s.audited("profile", s.profile)))
	mux.HandleFunc("POST /user/state", g(s.audited("user-state", s.userState)))
	mux.HandleFunc("POST /identity/remove", g(s.audited("remove-identity", s.removeIdentity)))
	mux.HandleFunc("GET /activity", g(s.activity))
	mux.HandleFunc("POST /group", g(s.audited("group", s.group)))
	mux.HandleFunc("GET /ls", g(s.ls))
	mux.HandleFunc("GET /inactive", g(s.inactive))
	mux.HandleFunc("GET /lookup", g(s.lookup))
	mux.HandleFunc("GET /recent", g(s.recent))
	mux.HandleFunc("GET /names", g(s.names))
	mux.HandleFunc("POST /subscribe", g(s.audited("subscribe", s.subscribe)))
	mux.HandleFunc("POST /subscriber/remove", g(s.audited("remove-subscriber", s.removeSubscriber)))
	mux.HandleFunc("POST /configure", g(s.audited("configure", s.configure)))
	mux.HandleFunc("GET /config", g(s.config))
	mux.HandleFunc("POST /secret", g(s.audited("set-secret", s.setSecret)))
	mux.HandleFunc("GET /secret", g(s.secret))
	mux.HandleFunc("POST /send", g(s.send))
	mux.HandleFunc("GET /consume", g(s.consume))
	mux.HandleFunc("POST /token", g(s.token))
	mux.HandleFunc("GET /debug", g(s.debugLog))
	mux.HandleFunc("POST /debug", g(s.audited("debug-log", s.debugLog)))
	mux.HandleFunc("POST /session", g(s.session))
	mux.HandleFunc("DELETE /session", g(s.endSession))
	// Enrolment needs no credential, because it is
	// where a credential comes from — requiring one would be a circle. It is
	// safe for the same reason: the signature *is* the credential, and only a
	// realm somebody vouches for can be enrolled into at all.
	// See docs/02-access.md#proving-possession.
	mux.HandleFunc("POST /enrol", func(w http.ResponseWriter, r *http.Request) {
		s.audited("enrol", s.enrol)(w, r, protocol.Name{})
	})
	// The API has no page, and a person who typed its address into a browser
	// wants the dashboard. `{$}` is the exact root and nothing below it: a
	// mistyped route stays the 404 it is, rather than becoming a redirect
	// that hides it.
	//
	// Permanent, because it is: this root will never grow a page of its own.
	// The cost is that a browser may stop asking, so moving the dashboard
	// afterwards is a thing to clear from a cache rather than to announce.
	// See docs/05-discovery.md#where-it-listens.
	if s.dash != "" {
		mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, s.dash, http.StatusMovedPermanently)
		})
	}
	return s.logged(mux)
}

// auth reads the caller out of the token, which is the whole of it: a token
// backs one principal, so there is nothing to cross-check and nothing on the
// wire to be somebody else with. See docs/02-access.md#what-a-call-carries.
func (s *Server) auth(next func(http.ResponseWriter, *http.Request, protocol.Name)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		who, pair, known := s.tokens.Credential(r.Header.Get(HeaderToken))
		if !known {
			s.refuse(w, http.StatusUnauthorized, "credential", "bad token")
			return
		}
		// A stored principal was a name when it was issued; parsing it back is
		// cheap and keeps every handler taking a Name rather than a string.
		name, err := protocol.ParseName(who)
		if err != nil {
			s.refuse(w, http.StatusUnauthorized, "credential", "bad token")
			return
		}
		// Who the credential was issued for, against who the name is now: a
		// credential left over from a name's previous holder answers for
		// nothing (docs/02-access.md#what-a-call-carries).
		if err := s.bus.CheckCredential(name.String(), pair); err != nil {
			s.refuse(w, http.StatusUnauthorized, "credential", "bad token")
			return
		}
		noteCaller(r, name)
		if err := s.bus.Authenticate(name.String()); err != nil {
			s.reply(w, nil, err)
			return
		}
		next(w, r, name)
	}
}

// onSocket is the local path: the account at the other end is known from
// which socket the connection arrived on, so nothing has to be sent and
// there is nothing to set up. The socket is a credential of the same kind as
// a token, not an exemption from having one.
// See docs/02-access.md#local-socket.
func (s *Server) onSocket(me protocol.Name) guard {
	return func(next func(http.ResponseWriter, *http.Request, protocol.Name)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			noteCaller(r, me)
			if err := s.bus.Authenticate(me.String()); err != nil {
				s.reply(w, nil, err)
				return
			}
			next(w, r, me)
		}
	}
}

// session turns the credential a person already holds into a shorter-lived
// one for a browser. The dashboard forwards what was typed once and keeps
// nothing of its own: the session lives here, in the process that holds state,
// so restarting the web child logs nobody out.
// See docs/05-discovery.md#signing-in.
func (s *Server) session(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	id, err := s.tokens.StartSession(caller.String())
	if err != nil {
		oops(w, err)
		return
	}
	ok(w, struct {
		Session string `json:"session"`
		Idle    string `json:"idle"`
	}{id, auth.IdleLife.String()})
}

// endSession drops the credential it arrived with. Nothing checks whose it
// was: holding it is the whole of the claim, and signing out is the holder's.
func (s *Server) endSession(w http.ResponseWriter, r *http.Request, _ protocol.Name) {
	s.tokens.EndSession(r.Header.Get(HeaderToken))
	ok(w, struct {
		Ended bool `json:"ended"`
	}{true})
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
	if !s.read(w, r, &in) {
		return
	}
	want, err := protocol.ParseName(in.Name)
	if err != nil {
		s.refuse(w, http.StatusBadRequest, "malformed", err.Error())
		return
	}
	issue := s.tokens.IssuePair
	if in.Rotate {
		issue = s.tokens.RotatePair
	}
	// Issued to somebody, never to nobody, and never to somebody who has
	// stopped being the one entitled to it. This is the operation that filled
	// the directory with names answering for nothing: a credential was minted
	// for a name the daemon held nothing else for, and the credential alone
	// let it call. Register the name, or create it as a user, first.
	//
	// Who may have it is asked in the registry rather than here, because here
	// it could only be asked before the mint and answered about a moment that
	// had passed: a record changes hands, and a caller that owned the name
	// when it asked would be handed the new owner's credential. The registry
	// decides and is still holding while the store is written.
	// See docs/02-access.md#getting-a-token.
	tok, err := s.bus.IssueFor(caller.String(), want.String(), issue)
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	ok(w, map[string]string{"name": want.String(), "token": tok})
}

// status also answers "who am I" — the one question a caller on its own
// socket cannot answer for itself, because it never stated a name.
// See docs/02-access.md#local-socket.
func (s *Server) status(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	owner := s.bus.DaemonOwner()
	ok(w, struct {
		core.Status
		You           string `json:"you"`
		Administrator bool   `json:"administrator,omitempty"`
		DaemonOwner   bool   `json:"daemon_owner,omitempty"`
	}{Status: s.bus.Status(), You: caller.String(), Administrator: s.bus.IsAdministrator(caller.String()), DaemonOwner: caller.String() == owner})
}

// subscribe puts the caller on a 📣 channel, or takes it off. The caller
// is the subscriber — there is no third name here, because a subscription
// puts messages in somebody's inbox and only they may ask for that.
//
// The field is `channel`, not `topic`: a channel is the record subscribed to,
// while an envelope's `topic` is a label on one message. One word for each.
// See docs/07-channels.md and docs/04-messaging.md#push-and-pull.
func (s *Server) subscribe(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Channel string `json:"channel"`
		Off     bool   `json:"off,omitempty"`
	}
	if !s.read(w, r, &in) {
		return
	}
	rec, err := s.bus.Subscribe(caller.String(), in.Channel, !in.Off)
	s.reply(w, rec, err)
}

func (s *Server) unregister(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct{ Name string }
	if !s.read(w, r, &in) {
		return
	}
	// The credential goes with the address, in the same operation. Keeping it
	// was how a removed name stayed reclaimable and protected from takeover;
	// that protection is [R1.2's](../../Plans/R1.2/README.md#removed-names),
	// and paying for it here cost a credential per throwaway address forever.
	//
	// Whether the name is a person, and so keeps its credential, is the
	// registry's to answer while it still holds — asked out here it was
	// answered after the record it depends on had already gone.
	err := s.bus.Unregister(in.Name, caller.String())
	s.reply(w, nil, err)
}

func (s *Server) register(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in protocol.Record
	if !s.read(w, r, &in) {
		return
	}
	in.Owner = caller.String()
	if r.Header.Get("If-None-Match") == "*" {
		rec, err := s.bus.RegisterNew(in)
		s.reply(w, rec, err)
		return
	}
	rec, err := s.bus.Register(in)
	s.reply(w, rec, err)
}

// configure stores a service's configuration. The body carries the name and
// the configuration itself, which stays opaque all the way down: it is only
// checked for being JSON. See docs/03-records.md#configuring-a-template.
func (s *Server) configure(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
	}
	if !s.read(w, r, &in) {
		return
	}
	rec, err := s.bus.Configure(in.Name, caller.String(), in.Config)
	s.reply(w, rec, err)
}

// setSecret stores a 📡's credential. The body carries the name and the
// secret, which stays opaque all the way down: nothing parses it.
// See docs/06-services.md#secrets.
func (s *Server) setSecret(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in struct {
		Name   string `json:"name"`
		Secret string `json:"secret"`
	}
	if !s.read(w, r, &in) {
		return
	}
	// The answer is the redacted record, so setting a secret hands back its
	// digest and never the bytes that were just sent.
	rec, err := s.bus.SetSecret(in.Name, caller.String(), in.Secret)
	s.reply(w, rec, err)
}

// secret hands one back to whoever the record's ACL admits. No listing and no
// record answer carries it, so this is the only route to the bytes.
func (s *Server) secret(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	value, err := s.bus.Secret(r.URL.Query().Get("name"), caller.String())
	if err != nil {
		s.reply(w, nil, err)
		return
	}
	// Written as a plain body, not wrapped in JSON: a credential goes into a
	// shell or an environment, and re-quoting it is a chance to mangle it.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(value))
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
// read. See docs/03-records.md.
func (s *Server) lookup(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	name := r.URL.Query().Get("name")
	rec, known := s.bus.Lookup(caller.String(), name)
	if !known {
		s.refuse(w, http.StatusNotFound, "unknown", "no such name: "+name)
		return
	}
	if at, has := s.tokens.LastUsed([]string{rec.Name})[rec.Name]; has {
		rec.LastUsed = &at
	}
	ok(w, rec)
}

// enrol is both halves of it: with no signature it hands back the nonce to
// sign, with one it checks the answer. Two calls because the bus has to be
// the one that says what gets signed — a challenge the caller chose proves
// nothing. See docs/01-identity-and-roles.md#registration.
func (s *Server) enrol(w http.ResponseWriter, r *http.Request, _ protocol.Name) {
	var in struct {
		Name      string `json:"name"`
		Nonce     string `json:"nonce"`
		Signature string `json:"signature"`
	}
	if !s.read(w, r, &in) {
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
// Each caller sees what it was party to; the daemon Owner sees the node
// (core/recent.go).
// See docs/05-discovery.md#dashboard.
func (s *Server) recent(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	ok(w, s.bus.Recent(caller.String()))
}

// names answers what this caller holds a credential for — their own name and
// the records they own — each with a fingerprint rather than the credential
// itself. Only ever your own: asking about somebody else's credentials is a
// question with no good answer. See docs/02-access.md#token-lifetime.
func (s *Server) names(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	held := s.tokens.Holds(s.bus.Owned(caller.String()))
	// Who it belongs to and what it is for. The registry knows, the
	// credential store does not, and a name list that cannot tell a person
	// from a service cannot be read (docs/05-discovery.md#dashboard).
	for i := range held {
		// A person first, and whatever they have registered second: an
		// identity may also hold a record of its own, and calling that a
		// service would put the one row this rule never touches under the
		// same word as the rows it does.
		if s.bus.IsPerson(held[i].Name) {
			held[i].Owner, held[i].Kind = held[i].Name, "person"
		} else if rec, known := s.bus.Lookup(caller.String(), held[i].Name); known {
			held[i].Owner, held[i].Kind = rec.Owner, rec.Kind
		} else {
			// Owned, credentialed, but nothing registered under it: an
			// address that was removed without its credential going too,
			// which is what this release stops happening.
			held[i].Owner, held[i].Kind = caller.String(), "unregistered"
		}
	}
	ok(w, held)
}

// ls answers the listing most recently active first: each record carries
// when its credential last authenticated a call, and those never used follow
// by name (docs/05-discovery.md#what-a-listing-answers).
func (s *Server) ls(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	recs := s.bus.List(caller.String(), r.URL.Query().Get("kind"))
	names := make([]string, len(recs))
	for i := range recs {
		names[i] = recs[i].Name
	}
	used := s.tokens.LastUsed(names)
	for i := range recs {
		if at, has := used[recs[i].Name]; has {
			recs[i].LastUsed = &at
		}
	}
	sort.SliceStable(recs, func(i, j int) bool {
		a, b := recs[i].LastUsed, recs[j].LastUsed
		if (a == nil) != (b == nil) {
			return a != nil
		}
		if a != nil && !a.Equal(*b) {
			return a.After(*b)
		}
		return recs[i].Name < recs[j].Name
	})
	ok(w, recs)
}

// inactive is the web face's one read-only view of inactive records
// (docs/constitution.md#common-record-fields).
func (s *Server) inactive(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	ok(w, s.bus.Inactive(caller.String()))
}

func (s *Server) send(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	var in protocol.Envelope
	if !s.read(w, r, &in) {
		return
	}
	in.From = caller.String()
	e, err := s.bus.Send(in)
	if err != nil {
		s.bus.RecordRefusal(in.To)
	}
	s.reply(w, e, err)
}

// consume long-polls an inbox. Which one, and whether it is filtered, is
// `inbox` selects the queue. Without it the caller reads its own. `topic` and
// `tag` only filter messages inside that queue; neither ever resolves a record
// name (docs/04-messaging.md#inbox-selection-and-filters).
//
// `share` says this reader is one of a pool, which is the only thing that
// lets a second unfiltered read wait beside it
// (docs/04-messaging.md#one-reader-per-inbox).
func (s *Server) consume(w http.ResponseWriter, r *http.Request, caller protocol.Name) {
	q := r.URL.Query()
	topic, tag := q.Get("topic"), q.Get("tag")
	filtered := topic != "" || tag != ""
	inbox := q.Get("inbox")
	if !q.Has("inbox") {
		inbox = caller.String()
	}

	wait := 30 * time.Second
	if v := q.Get("wait"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			wait = min(d, maxWait)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), wait)
	defer cancel()

	e, err := s.bus.ConsumeAs(ctx, caller.String(), inbox, topic, tag, filtered, q.Has("share"))
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		s.bus.RecordRefusal(inbox)
	}
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
	{core.ErrProfile, http.StatusBadRequest, "malformed"},
	{core.ErrInactive, http.StatusForbidden, "suspended"},
	// Counted as a credential refusal, not as a second reason: the caller is
	// being told the same thing a bad token is told, which is that what they
	// presented does not make them anybody.
	{core.ErrNoPrincipal, http.StatusUnauthorized, "credential"},
	{core.ErrBadName, http.StatusBadRequest, "malformed"},
	{core.ErrOverflow, http.StatusBadRequest, "malformed"},
	{core.ErrKind, http.StatusBadRequest, "malformed"},
	{core.ErrConfig, http.StatusBadRequest, "malformed"},
	{core.ErrSecret, http.StatusBadRequest, "malformed"},
	{core.ErrEnv, http.StatusBadRequest, "malformed"},
	{core.ErrNoSecret, http.StatusNotFound, "unknown"},
	{core.ErrReceipt, http.StatusBadRequest, "malformed"},
	{core.ErrTTL, http.StatusBadRequest, "malformed"},
	{core.ErrWait, http.StatusBadRequest, "malformed"},
	{core.ErrBound, http.StatusBadRequest, "malformed"},
	{core.ErrNotOwner, http.StatusForbidden, "acl"},
	{core.ErrExists, http.StatusPreconditionFailed, "name-taken"},
	{core.ErrBusy, http.StatusConflict, "busy"},
	{core.ErrPrivate, http.StatusForbidden, "acl"},
	{core.ErrNotAllow, http.StatusForbidden, "acl"},
	{core.ErrPersonal, http.StatusBadRequest, "malformed"},
	{core.ErrEnrol, http.StatusForbidden, "enrolment"},
	{core.ErrNoRemoval, http.StatusBadRequest, "malformed"},
	{core.ErrUnknown, http.StatusNotFound, "unknown"},
	// Not 503: a full inbox is the sender outrunning the reader, not the
	// service being unavailable — and 503 is the service's own answer for
	// that (Plans/R1.1/records.md#coming-back-in-a-moment-is-not-one-of-them).
	{core.ErrFull, http.StatusTooManyRequests, "full"},
	{core.ErrTwoReads, http.StatusConflict, "second-reader"},
}

// Reasons is the closed set of refusal reasons, sorted
// (docs/05-discovery.md#refusals). A status answer carries only the reasons
// that have happened, and a face showing refusals has to know which zeros to
// draw: a supported reason absent from a successful status is a measured zero,
// not a gap in observation. Derived from the table above so the two cannot
// drift — a reason added there appears here without being named twice.
func Reasons() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, c := range codes {
		if !seen[c.kind] {
			seen[c.kind] = true
			out = append(out, c.kind)
		}
	}
	sort.Strings(out)
	return out
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
			s.refuse(w, c.code, c.kind, err.Error())
			return
		}
	}
	oops(w, err)
}

// read decodes a request body, and a body it cannot decode is a refusal like
// any other — the caller asked for something and was turned away. This is the
// shared decoding path for JSON request bodies.
func (s *Server) read(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		s.refuse(w, http.StatusBadRequest, "malformed", "bad json: "+err.Error())
		return false
	}
	return true
}

// readStrict is for narrow operations whose body is also an authority
// boundary. Ignoring a protected field there would make a request appear to
// change something the operation can never change.
func (s *Server) readStrict(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		s.refuse(w, http.StatusBadRequest, "malformed", "bad json: "+err.Error())
		return false
	}
	return true
}

func ok(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// refuse counts an endpoint's refusal once, then writes it. Handlers must use
// this rather than the bare answer writer for caller errors. Router rejections
// and internal failures do not count (docs/05-discovery.md#refusals).
// The reason cannot be inferred from the status: 403 and 409 each cover
// several distinct reasons.
func (s *Server) refuse(w http.ResponseWriter, code int, reason, msg string) {
	s.bus.Refuse(reason)
	answer(w, code, msg)
}

// oops is the other half: ours rather than the caller's, always 500, never
// counted. Refusing a caller and failing them are different things to be told
// about.
func oops(w http.ResponseWriter, err error) {
	answer(w, http.StatusInternalServerError, err.Error())
}

func answer(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
