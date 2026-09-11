// agent-busd: one process, two listeners, everything in memory.
// PoC scope: docs/12-stages.md#poc.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/auth"
	"github.com/parf/ai-agent-bus/internal/core"
	"github.com/parf/ai-agent-bus/internal/dump/jsonfile"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/file"
)

func main() {
	var (
		addr   = flag.String("addr", env("AGENT_BUS_ADDR", "127.0.0.1:7777"), "TCP listen address — loopback only in PoC")
		sock   = flag.String("socket", env("AGENT_BUS_SOCKET", api.DefaultSocket()), "unix socket path")
		tokenF = flag.String("token-file", env("AGENT_BUS_TOKEN_FILE", defaultTokenFile()), "token store; created if absent")
		owner  = flag.String("owner", env("AGENT_BUS_OWNER", defaultOwner()), "the principal this daemon belongs to")
		dumpF  = flag.String("dump-file", env("AGENT_BUS_DUMP_FILE", defaultDumpFile()), "where the queues and stats are snapshotted")
		every  = flag.Duration("dump-every", time.Minute, "how often to snapshot while running; 0 turns the periodic dumper off")
		users  accounts
	)
	flag.Var(&users, "user", "a local account and the principal it is: `account=user@realm`; repeatable")
	flag.Parse()

	tokens, err := auth.Load(file.NewTokens(*tokenF), *owner)
	if err != nil {
		log.Fatalf("token: %v", err)
	}
	me, err := protocol.ParseName(*owner)
	if err != nil {
		log.Fatalf("owner: %v", err)
	}
	// Queues, stats and the registry are memory; the snapshot is what a
	// restart reads back. See docs/04-messaging.md#durability.
	bus, snap := core.New(), jsonfile.New(*dumpF)
	if s, found, err := snap.Load(); err != nil {
		log.Fatalf("dump %s: %v", *dumpF, err)
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
	face := api.New(bus, tokens, me.String())

	// Plaintext bodies and a master token: loopback or an SSH tunnel, never a
	// public interface. See docs/12-stages.md#poc.
	if err := loopbackOnly(*addr); err != nil {
		log.Fatal(err)
	}
	tcp, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen %s: %v", *addr, err)
	}
	// 0711: it holds one socket per account, so everyone must be able to
	// walk through it to their own. Nobody can list it, and each socket is
	// 0600 to its owner. See docs/02-access.md#local-socket.
	if err := os.MkdirAll(filepath.Dir(*sock), 0o711); err != nil {
		log.Fatalf("socket dir: %v", err)
	}
	if err := os.Chmod(filepath.Dir(*sock), 0o711); err != nil {
		log.Fatalf("socket dir: %v", err)
	}
	if err := clearStaleSocket(*sock); err != nil {
		log.Fatal(err)
	}
	unix, err := net.Listen("unix", *sock)
	if err != nil {
		log.Fatalf("listen %s: %v", *sock, err)
	}
	// The socket is the credential's hiding place, so the mode is not advice.
	if err := os.Chmod(*sock, 0o600); err != nil {
		log.Fatalf("chmod %s: %v", *sock, err)
	}

	var srvs []*http.Server
	serve := func(l net.Listener, h http.Handler) {
		srv := &http.Server{
			Handler: h,
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
	shared := face.Handler()
	serve(tcp, shared)
	serve(unix, shared)

	// One socket per local account, so the daemon knows who is calling with
	// nothing for anyone to configure. The account running the daemon is a
	// user of it like any other, so it gets one without being asked for.
	// See docs/02-access.md#local-socket.
	users.add(ownerAccount(), me)
	mine := []string{*sock}
	for _, u := range users.list() {
		path := filepath.Join(filepath.Dir(*sock), "user-"+u.account+".sock")
		l, err := u.listen(path)
		if err != nil {
			log.Fatalf("socket for %s: %v", u.account, err)
		}
		mine = append(mine, path)
		serve(l, face.HandlerFor(u.name))
	}
	log.Printf("agent-busd on http://%s and %s for %s, %d user sockets (tokens %s)",
		*addr, *sock, me, len(users.list()), *tokenF)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	// The periodic dumper bounds what an untimely death costs to one
	// interval; without it a crash loses everything since start.
	if *every > 0 {
		tick := time.NewTicker(*every)
		defer tick.Stop()
		go func() {
			for range tick.C {
				save(false)
			}
		}()
	}
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, srv := range srvs {
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}
	// After the listeners are closed, so nothing arrives between the
	// snapshot and the last reply. See docs/04-messaging.md#durability.
	save(true)
	for _, p := range mine {
		os.Remove(p)
	}
	log.Print("stopped")
}

// accounts is the local account -> principal mapping setup writes down.
// See docs/09-setup.md#local-users.
type accounts struct {
	seen map[string]bool
	all  []account
}

type account struct {
	account string
	name    protocol.Name
	uid     int
}

func (a *accounts) String() string { return fmt.Sprintf("%d accounts", len(a.all)) }

func (a *accounts) Set(v string) error {
	who, principal, ok := strings.Cut(v, "=")
	if !ok || who == "" || principal == "" {
		return fmt.Errorf("want account=user@realm, got %q", v)
	}
	name, err := protocol.ParseName(principal)
	if err != nil {
		return err
	}
	u, err := user.Lookup(who)
	if err != nil {
		return fmt.Errorf("no local account %q: %w", who, err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("uid of %q: %w", who, err)
	}
	a.add2(account{account: who, name: name, uid: uid})
	return nil
}

// add is Set for a mapping the daemon already knows is good.
func (a *accounts) add(who string, name protocol.Name) {
	uid := os.Getuid()
	if u, err := user.Lookup(who); err == nil {
		if n, err := strconv.Atoi(u.Uid); err == nil {
			uid = n
		}
	}
	a.add2(account{account: who, name: name, uid: uid})
}

// add2 keeps the first mapping for an account: a later one would move
// somebody's socket out from under them.
func (a *accounts) add2(x account) {
	if a.seen == nil {
		a.seen = map[string]bool{}
	}
	if a.seen[x.account] {
		return
	}
	a.seen[x.account] = true
	a.all = append(a.all, x)
}

func (a *accounts) list() []account { return a.all }

// listen opens one account's socket: theirs to reach, nobody else's to read.
// The chown needs CAP_CHOWN, which by the design belongs to a supervisor
// this daemon does not have yet (docs/11-processes.md); until it does, a
// failure here is said out loud rather than left to look like it worked.
func (a account) listen(path string) (net.Listener, error) {
	if err := clearStaleSocket(path); err != nil {
		return nil, err
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, err
	}
	if err := os.Chown(path, a.uid, -1); err != nil && os.Getuid() != a.uid {
		log.Printf("WARNING: %s stays this account's, not %s's: %v — it needs CAP_CHOWN",
			path, a.account, err)
	}
	return l, nil
}

// ownerAccount is the local account the daemon runs as.
func ownerAccount() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "agent-bus"
}

// clearStaleSocket removes a leftover socket, and refuses to touch anything
// else: a regular file at that path is a mistake, and a live daemon there is
// somebody else's.
func clearStaleSocket(path string) error {
	fi, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%s exists and is not a socket; refusing to remove it", path)
	}
	if c, err := net.DialTimeout("unix", path, 200*time.Millisecond); err == nil {
		c.Close()
		return fmt.Errorf("%s is already served by a running daemon", path)
	}
	return os.Remove(path)
}

func loopbackOnly(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("bad -addr %q: %w", addr, err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("-addr %q is not loopback: PoC has no encryption, so it does not bind a public interface", addr)
	}
	return nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// defaultOwner is the account running the daemon, vouched for by this host —
// the `user@host` form in docs/01-identity.md#names.
func defaultOwner() string {
	who := "agent-bus"
	if u, err := user.Current(); err == nil && u.Username != "" {
		who = u.Username
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "localhost"
	}
	if i := strings.IndexByte(host, '.'); i > 0 {
		host = host[:i]
	}
	return who + "@" + host
}

func defaultDumpFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "agent-bus", "dump.json")
}

func defaultTokenFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "agent-bus", "token")
}
