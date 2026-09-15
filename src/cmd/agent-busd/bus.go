// The bus child: registry, queues and delivery. It holds the store and the
// dump, and nothing else holds them (docs/11-processes.md#what-is-shared).
// It has no capability, cannot create a socket and cannot chown one — it is
// handed listeners that already exist.
package main

import (
	"context"
	"errors"
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
	"github.com/parf/ai-agent-bus/internal/core"
	dirfile "github.com/parf/ai-agent-bus/internal/directory/file"
	"github.com/parf/ai-agent-bus/internal/directory/github"
	"github.com/parf/ai-agent-bus/internal/dump/jsonfile"
	"github.com/parf/ai-agent-bus/internal/ports"
	"github.com/parf/ai-agent-bus/internal/proctitle"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/signature/sshkeygen"
	"github.com/parf/ai-agent-bus/internal/store/file"
)

func runBus(c config) {
	var calls atomic.Uint64
	defer proctitle.Start("agent-busd", "bus", &calls)()
	tokens, err := auth.Load(file.NewTokens(c.tokenF), c.owner)
	if err != nil {
		log.Fatalf("token: %v", err)
	}
	me, err := protocol.ParseName(c.owner)
	if err != nil {
		log.Fatalf("owner: %v", err)
	}
	// Queues, stats and the registry are memory; the snapshot is what a
	// restart reads back. See docs/04-messaging.md#durability.
	bus, snap := core.New(), jsonfile.New(c.dumpF)
	if s, found, err := snap.Load(); err != nil {
		log.Fatalf("dump %s: %v", c.dumpF, err)
	} else if found {
		if !s.Clean {
			log.Printf("WARNING: the last run did not stop cleanly; anything queued after %s is gone",
				s.At.Format(time.RFC3339))
		}
		bus.Restore(s)
	}
	// Written straight away and not clean: the next start needs to tell a
	// first one from one that follows a death, and only a file on disk can.
	save := func(clean bool) {
		s := bus.Snapshot()
		s.Clean = clean
		if err := snap.Save(s); err != nil {
			log.Printf("dump: %v", err)
		}
	}
	save(false)
	// The daemon's owner holds master without being listed: they installed
	// it, and the setup user is the admin. See docs/01-identity.md#acl.
	bus.Masters(append([]string{me.String()}, c.hold...))
	// A realm somebody vouches for can only be entered by proving you hold
	// a key it publishes. Realms nobody vouches for stay open, as they were.
	// See docs/01-identity.md#registration.
	dirs := map[string]ports.Directory{}
	for _, v := range c.vouch {
		realm, what, ok := strings.Cut(v, "=")
		if !ok || realm == "" || what == "" {
			log.Fatalf("--directory wants realm=github or realm=/path/to/keys, not %q", v)
		}
		if what == "github" {
			dirs[realm] = github.New()
		} else {
			dirs[realm] = dirfile.New(what)
		}
	}
	bus.Directories(dirs, sshkeygen.New())
	face := api.New(bus, tokens, me.String())
	face.Dashboard(c.dash)

	var srvs []*http.Server
	serve := func(l net.Listener, h http.Handler) {
		srv := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				h.ServeHTTP(w, r)
			}),
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
		if in.who == "" {
			serve(in.l, face.Handler())
			continue
		}
		name, err := protocol.ParseName(in.who)
		if err != nil {
			log.Fatalf("fd for %q: %v", in.who, err)
		}
		serve(in.l, face.HandlerFor(name))
	}
	log.Printf("bus serving %d listeners for %s (tokens %s)", len(srvs), me, c.tokenF)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	// The periodic dumper bounds what an untimely death costs to one interval.
	if c.every > 0 {
		tick := time.NewTicker(c.every)
		defer tick.Stop()
		go func() {
			for range tick.C {
				save(false)
			}
		}()
	}
	bus.SampleActivity(time.Now())
	activityTick := time.NewTicker(time.Minute)
	activityDone := make(chan struct{})
	go func() {
		for {
			select {
			case at := <-activityTick.C:
				bus.SampleActivity(at)
			case <-activityDone:
				return
			}
		}
	}()
	<-stop
	activityTick.Stop()
	close(activityDone)
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

// a listener the supervisor opened, and the principal it speaks for — empty
// for the shared ones, where the caller names themselves.
type inlet struct {
	l   net.Listener
	who string
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
		in = append(in, inlet{l: l, who: who})
	}
	return in
}
