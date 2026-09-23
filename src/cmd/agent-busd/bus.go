// The bus child: registry, queues and delivery. It holds the store and the
// dump, and nothing else holds them (docs/11-processes.md#what-is-shared).
// It has no capability, cannot create a socket and cannot chown one — it is
// handed listeners that already exist.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/callstats"
	"github.com/parf/ai-agent-bus/internal/core"
	dirfile "github.com/parf/ai-agent-bus/internal/directory/file"
	"github.com/parf/ai-agent-bus/internal/directory/github"
	"github.com/parf/ai-agent-bus/internal/journal"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/proctitle"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/signature/sshkeygen"
	"github.com/parf/ai-agent-bus/internal/store/sqlite"
)

func runBus(c config) {
	var calls atomic.Uint64
	callHistory := callstats.New(&calls)
	defer proctitle.Start("agent-busd", "bus", &calls)()
	me, err := requiredOwner(c.owner)
	if err != nil {
		log.Fatalf("owner: %v", err)
	}
	// Every durable entity is in the database, taken exclusively: a second
	// daemon on the same file cannot serve (docs/constitution.md#persistence-and-loading).
	// Queues live in memory between flushes. See docs/04-messaging.md#durability.
	st, err := sqlite.Open(c.db, c.create)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()
	// The three logs (docs/constitution.md#logs). Opened before anything can
	// need reporting, and closed last.
	logs, err := journal.Open(c.logDir, c.debugLog)
	if err != nil {
		log.Fatalf("logs: %v", err)
	}
	defer logs.Close()
	bus := core.New()
	bus.Journal(logs)
	if s, err := st.Load(); err != nil {
		log.Fatalf("database %s: %v", c.db, err)
	} else {
		if !s.Clean && !s.At.IsZero() {
			msg := fmt.Sprintf("the last run did not stop cleanly; queue changes after %s are gone", s.At.Format(time.RFC3339))
			log.Print("WARNING: " + msg)
			logs.Report(ports.Warning, msg)
		}
		bus.Restore(s)
	}
	// Bound before the first write, so setup's seed below commits like any
	// other change.
	bus.Persistence(st)
	if err := bus.EstablishDaemonOwner(me.String()); err != nil {
		log.Fatalf("owner: %v", err)
	}
	activeAccounts := map[string]string{}
	if raw := os.Getenv(accountsEnv); raw == "" {
		log.Fatalf("%s is empty: the bus is started by the supervisor, not by hand", accountsEnv)
	} else if err := json.Unmarshal([]byte(raw), &activeAccounts); err != nil {
		log.Fatalf("%s: %v", accountsEnv, err)
	}
	if err := bus.EstablishAccounts(activeAccounts); err != nil {
		log.Fatalf("local accounts: %v", err)
	}
	owner := bus.DaemonOwner()
	tokens, err := auth.Load(st.Tokens(), owner)
	if err != nil {
		log.Fatalf("token: %v", err)
	}
	// Written straight away and not clean: the next start needs to tell a
	// first one from one that follows a death, and only the database can.
	save := func(clean bool) {
		if err := bus.FlushQueues(clean); err != nil {
			log.Printf("flush: %v", err)
			logs.Report(ports.Error, "queue flush failed: "+err.Error())
		}
		// Last-use times ride the same cadence: one batch, never a write per
		// authenticated call (docs/02-access.md#token-lifetime).
		if err := tokens.FlushUsed(); err != nil {
			log.Printf("credential use: %v", err)
			logs.Report(ports.Error, "credential last-use flush failed: "+err.Error())
		}
	}
	save(false)
	// A realm somebody vouches for can only be entered by proving you hold
	// a key it publishes. Realms nobody vouches for stay open, as they were.
	// See docs/01-identity-and-roles.md#registration.
	if err := configureDirectories(bus, c.vouch, github.New()); err != nil {
		log.Fatal(err)
	}
	face := api.New(bus, tokens, owner)
	face.Journal(logs)
	face.LocalAccounts(ownerAccount(), validateLocalAccount)
	face.Dashboard(c.dash)
	face.Calls(callHistory.Snapshot)

	// A record whose owner is not a User was ignored at load and reported
	// (docs/constitution.md#persistence-and-loading), so its name is not
	// known here; the credential sweep below is what takes a credential that
	// answered for it.

	// A credential whose name has no record and no registered user answers for
	// nothing, and is dropped here — at start, at a known moment, never by
	// expiry (docs/02-access.md#ownerless-credentials).
	//
	// After Restore and owner establishment, so the records and users it asks
	// about are the durable ones. Either way round it would sweep what it is
	// meant to keep.
	if swept := bus.Ownerless(tokens.Names()); len(swept) > 0 {
		for _, name := range swept {
			if err := tokens.Forget(name); err != nil {
				log.Printf("ownerless %s: %v", name, err)
			}
		}
		log.Printf("dropped %d credentials that answered for nothing", len(swept))
	}
	// The snapshot written at line one of this run predates both sweeps, so a
	// restart that died before the next periodic save would read the wreckage
	// back and do this again. Written here, a start that then dies repeats
	// nothing — as far as the write succeeds. save logs and continues, so this
	// is best effort like every other snapshot; what it is not is skipped.
	save(false)

	callHistory.Sample(time.Now())
	var srvs []*http.Server
	serve := func(l net.Listener, h http.Handler) {
		srv := &http.Server{
			Handler: countCalls(&calls, h),
			// Long-poll consume holds a request open, so there is no write
			// deadline; the header and idle deadlines cost nothing.
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       2 * time.Minute,
		}
		srvs = append(srvs, srv)
		go func() {
			if err := srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("serve %s: %v", l.Addr(), err)
			}
		}()
	}
	for _, in := range inherited() {
		h, err := socketHandler(bus, face, in)
		if err != nil {
			log.Fatal(err)
		}
		if h == nil {
			log.Printf("socket for %s is not served: its account mapping was ignored at load", in.who)
			in.l.Close()
			continue
		}
		serve(in.l, h)
	}
	log.Printf("bus serving %d listeners for %s (database %s)", len(srvs), owner, c.db)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	// The periodic dumper bounds what an untimely death costs to one interval.
	stopSnapshots := func() {}
	if c.every > 0 {
		tick := time.NewTicker(c.every)
		done, stopped := make(chan struct{}), make(chan struct{})
		stopSnapshots = func() { tick.Stop(); close(done); <-stopped }
		go func() {
			defer close(stopped)
			for {
				select {
				case <-tick.C:
					save(false)
				case <-done:
					return
				}
			}
		}()
	}
	clockDone := make(chan struct{})
	go everyMinute(clockDone, time.Now, time.After, onMinute(callHistory, bus))
	<-stop
	// Join the periodic writer before the final clean checkpoint; no later
	// tick may replace it with an unclean snapshot during shutdown.
	stopSnapshots()
	close(clockDone)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, srv := range srvs {
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}
	// After the listeners are closed, so nothing arrives between the
	// snapshot and the last reply. See docs/04-messaging.md#durability.
	// The sockets themselves are the supervisor's to remove.
	save(true)
	log.Print("bus stopped")
}

// everyMinute calls fn at every minute of the clock — :00 seconds, not a
// minute from start — until done closes. The wait is recomputed from the
// clock each time, so a late wake or a clock step costs one reading, not the
// cadence. now and after are the clock, replaceable by a test.
func everyMinute(done <-chan struct{}, now func() time.Time, after func(time.Duration) <-chan time.Time, fn func(time.Time)) {
	for {
		t := now()
		select {
		case <-after(t.Truncate(time.Minute).Add(time.Minute).Sub(t)):
			fn(now())
		case <-done:
			return
		}
	}
}

// onMinute is what the bus does at each minute of the clock: one reading of
// the call counter, so "Calls, minute" is a minute and 61 readings an hour
// (docs/05-discovery.md#what-a-node-says-about-itself), and the activity
// tick, which closes a slot at :00, :10 … :50 local time
// (docs/05-discovery.md#activity-history).
func onMinute(calls *callstats.History, bus *core.Bus) func(time.Time) {
	return func(at time.Time) {
		calls.Sample(at)
		bus.TickActivity(at)
	}
}

type githubDirectory interface {
	ports.Directory
	ports.ProfileDirectory
}

// configureDirectories keeps two independent choices independent. Directory
// flags decide which realms require key-possession enrolment. Public GitHub
// profile metadata is optional decoration for any user profile and is always
// available through the trusted adapter, even when no realm is GitHub-backed.
func configureDirectories(bus *core.Bus, specs []string, profiles githubDirectory) error {
	dirs := map[string]ports.Directory{}
	for _, v := range specs {
		realm, what, ok := strings.Cut(v, "=")
		if !ok || realm == "" || what == "" {
			return fmt.Errorf("--directory wants realm=github or realm=/path/to/keys, not %q", v)
		}
		if what == "github" {
			dirs[realm] = profiles
		} else {
			dirs[realm] = dirfile.New(what)
		}
	}
	bus.Directories(dirs, sshkeygen.New())
	bus.ProfileDirectory(profiles)
	return nil
}

// socketHandler is what one inherited listener serves, or nil when it is not
// to be served at all.
func socketHandler(bus *core.Bus, face *api.Server, in inlet) (http.Handler, error) {
	// The daemon account's socket answers as the daemon Owner of each
	// request, which a transfer moves, not as the setup seed on the command
	// line.
	if in.owner {
		return face.HandlerForOwner(), nil
	}
	if in.who == "" {
		return face.Handler(), nil
	}
	name, err := protocol.ParseName(in.who)
	if err != nil {
		return nil, fmt.Errorf("fd for %q: %w", in.who, err)
	}
	// A mapping ignored at load names nobody; its socket is not served, or it
	// would answer for whoever took the name next (docs/02-access.md#local-socket).
	if bus.Unserved(name.String()) {
		return nil, nil
	}
	return face.HandlerFor(name), nil
}

// a listener the supervisor opened, and the principal it speaks for — empty
// for the shared ones, where the caller names themselves.
type inlet struct {
	l   net.Listener
	who string
	// owner marks the daemon account's socket, which answers as the durable
	// daemon Owner rather than a name fixed at supervisor start.
	owner bool
}

// inherited turns the fds the supervisor passed into listeners. They arrive
// at 3 upwards in the order AGENT_BUS_FDS names them, which is the whole
// contract between the two processes (docs/11-processes.md#what-is-shared).
func inherited() []inlet {
	spec := os.Getenv(fdsEnv)
	if spec == "" {
		log.Fatalf("%s is empty: the bus is started by the supervisor, not by hand", fdsEnv)
	}
	var in []inlet
	for i, what := range strings.Split(spec, ",") {
		f := os.NewFile(uintptr(3+i), what)
		l, err := net.FileListener(f)
		if err != nil {
			log.Fatalf("fd %d (%s): %v", 3+i, what, err)
		}
		f.Close() // FileListener dups; this one has done its work
		who := ""
		if rest, ok := strings.CutPrefix(what, "user:"); ok {
			who = rest
		}
		in = append(in, inlet{l: l, who: who, owner: what == "owner:"})
	}
	return in
}

// Count admission, not completion: a long poll already occupies the daemon.
// Every listener uses this wrapper and the process title reads the same atomic.
func countCalls(calls *atomic.Uint64, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		h.ServeHTTP(w, r)
	})
}
