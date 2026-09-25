// agent-busd: a supervisor that owns the listeners, and a bus child that
// serves them. One binary; the role comes from the environment, because a
// second binary would be a second thing to install for no gain.
// See docs/11-processes.md#the-rule.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/dashboard"
	"github.com/parf/ai-agent-bus/internal/proctitle"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/store/sqlite"
	"github.com/parf/ai-agent-bus/internal/version"
)

// What the supervisor tells a child it is, and which listener is which fd.
// Both are read once at start and never again.
const (
	roleEnv     = "AGENT_BUS_ROLE"
	fdsEnv      = "AGENT_BUS_FDS"
	accountsEnv = "AGENT_BUS_ACCOUNTS"
	roleBus     = "bus"
)

type config struct {
	addr, sock, owner, db string
	logDir                string
	debugLog              bool
	create, init          bool
	dash                  string
	every                 time.Duration
	users                 accounts
	vouch                 values
}

func main() {
	if version.Print() {
		return
	}
	var c config
	// -web ran the Go dashboard as a child until 0.8.50; the web face is now
	// its own unit (docs/11-processes.md#the-web-face). Accepted and ignored,
	// so a unit written before then still starts.
	flag.Bool("web", false, "ignored: the web face is its own agent-bus-web unit since 0.8.50")
	flag.StringVar(&c.addr, "addr", "127.0.0.1:6767", "TCP listen address, any interface; plain HTTP, so off loopback tokens and bodies cross the network unencrypted. AGENT_BUS_ADDR is the CLI's socket, never this")
	flag.StringVar(&c.sock, "socket", env("AGENT_BUS_SOCKET", api.DefaultSocket()), "unix socket path")
	flag.StringVar(&c.owner, "owner", env("AGENT_BUS_OWNER", ""), "initial daemon owner (required; later transfers are durable)")
	flag.StringVar(&c.db, "db", env("AGENT_BUS_DB", defaultDB()), "the SQLite database holding every durable entity, credential and queue")
	flag.BoolVar(&c.create, "create", false, "create the database when it does not exist; without it a missing database refuses the start")
	flag.StringVar(&c.logDir, "log-dir", env("AGENT_BUS_LOG_DIR", ""), "where debug.log, audit.log and error.log are written; defaults to logs/ beside the database")
	flag.BoolVar(&c.debugLog, "debug-log", false, "write debug.log, a line per request, from the start; the daemon owner can also switch it at run time")
	flag.BoolVar(&c.init, "init", false, "create the database if it is absent, check it, and exit: what setup runs before the first start")
	flag.DurationVar(&c.every, "flush-every", time.Minute, "how often queue contents and counters are saved while running; 0 saves them only at a graceful stop")
	flag.StringVar(&c.dash, "dashboard", env("AGENT_BUS_DASHBOARD", dashboard.URL), "where a browser opening the API `url` is sent; empty serves no root at all")
	flag.Var(&c.vouch, "directory", "a realm and what vouches for it: `realm=github` or `realm=/path/to/keys`; repeatable")
	flag.Var(&c.users, "user", "a local account and the principal it is: `account=user[@realm]`; repeatable")
	flag.Parse()
	if c.logDir == "" {
		c.logDir = filepath.Join(filepath.Dir(c.db), "logs")
	}
	if _, err := requiredOwner(c.owner); err != nil {
		log.Fatalf("owner: %v", err)
	}

	if c.init {
		if err := initDatabase(c.db); err != nil {
			log.Fatalf("database: %v", err)
		}
		fmt.Printf("database ready: %s\n", c.db)
		return
	}
	if os.Getenv(roleEnv) == roleBus {
		runBus(c)
		return
	}
	defer proctitle.Start("agent-busd", "supervisor", nil)()
	runSupervisor(c)
}

// initDatabase is the one explicit way a database comes into being on an
// installed node; the unit never passes -create, so a database that vanished
// refuses the start instead of being replaced by an empty one.
func initDatabase(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	st, err := sqlite.Open(path, true)
	if err != nil {
		return err
	}
	defer st.Close()
	_, err = st.Load()
	return err
}

func requiredOwner(value string) (protocol.Name, error) {
	name, err := protocol.ParseName(value)
	if err != nil {
		return protocol.Name{}, fmt.Errorf("required --owner name: %w", err)
	}
	return name, nil
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
		return fmt.Errorf("want account=user[@realm], got %q", v)
	}
	name, err := protocol.ParseName(principal)
	if err != nil {
		return err
	}
	uid, err := localAccountUID(who)
	if err != nil {
		return err
	}
	a.add2(account{account: who, name: name, uid: uid})
	return nil
}

func localAccountUID(who string) (int, error) {
	u, err := user.Lookup(who)
	if err != nil {
		return 0, fmt.Errorf("no local account %q: %w", who, err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, fmt.Errorf("uid of %q: %w", who, err)
	}
	return uid, nil
}

func validateLocalAccount(who string) error {
	_, err := localAccountUID(who)
	return err
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

func (a *accounts) mapping() map[string]string {
	out := make(map[string]string, len(a.all))
	for _, entry := range a.all {
		out[entry.account] = entry.name.String()
	}
	return out
}

// listen opens one account's socket: theirs to reach, nobody else's to read.
// The chown needs CAP_CHOWN, which is the supervisor's and no child's
// (docs/11-processes.md#why-the-supervisor-holds-cap_chown); where it is
// missing the failure is said out loud rather than left to look like it worked.
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
	return "agent-busd"
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

// offLoopback says whether addr reaches past this machine: an empty host,
// a wildcard, a name or any non-loopback IP does.
func offLoopback(addr string) (bool, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false, fmt.Errorf("bad -addr %q: %w", addr, err)
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback(), nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// values is a small repeatable string flag used for directory sources.
type values []string

func (m *values) String() string     { return strings.Join(*m, ",") }
func (m *values) Set(v string) error { *m = append(*m, v); return nil }

func defaultDB() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "agent-bus", "agent-bus.db")
}
