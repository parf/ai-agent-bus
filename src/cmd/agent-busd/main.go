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
	addr, sock, tokenF, owner, dumpF string
	dash                             string
	every                            time.Duration
	users                            accounts
	hold, vouch                      masters
	web                              bool
}

func main() {
	if version.Print() {
		return
	}
	var c config
	flag.StringVar(&c.addr, "addr", env("AGENT_BUS_ADDR", "127.0.0.1:6767"), "TCP listen address — loopback only")
	flag.StringVar(&c.sock, "socket", env("AGENT_BUS_SOCKET", api.DefaultSocket()), "unix socket path")
	flag.StringVar(&c.tokenF, "token-file", env("AGENT_BUS_TOKEN_FILE", defaultTokenFile()), "token store; created if absent")
	flag.StringVar(&c.owner, "owner", env("AGENT_BUS_OWNER", ""), "initial daemon owner (required; later transfers are durable)")
	flag.StringVar(&c.dumpF, "dump-file", env("AGENT_BUS_DUMP_FILE", defaultDumpFile()), "where the queues and stats are snapshotted")
	flag.DurationVar(&c.every, "dump-every", time.Minute, "how often to snapshot while running; 0 turns the periodic dumper off")
	flag.BoolVar(&c.web, "web", false, "run the dashboard as a child too (docs/05-discovery.md#dashboard)")
	flag.StringVar(&c.dash, "dashboard", env("AGENT_BUS_DASHBOARD", dashboard.URL), "where a browser opening the API `url` is sent; empty serves no root at all")
	flag.Var(&c.vouch, "directory", "a realm and what vouches for it: `realm=github` or `realm=/path/to/keys`; repeatable")
	flag.Var(&c.hold, "master", "a principal that reaches every service which has not refused it: `user@realm`; repeatable")
	flag.Var(&c.users, "user", "a local account and the principal it is: `account=user@realm`; repeatable")
	flag.Parse()
	if _, err := requiredOwner(c.owner); err != nil {
		log.Fatalf("owner: %v", err)
	}

	if os.Getenv(roleEnv) == roleBus {
		runBus(c)
		return
	}
	defer proctitle.Start("agent-busd", "supervisor", nil)()
	runSupervisor(c)
}

func requiredOwner(value string) (protocol.Name, error) {
	name, err := protocol.ParseName(value)
	if err != nil {
		return protocol.Name{}, fmt.Errorf("required --owner user@realm: %w", err)
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
		return fmt.Errorf("want account=user@realm, got %q", v)
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

func loopbackOnly(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("bad -addr %q: %w", addr, err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("-addr %q is not loopback: bodies are plaintext, so the daemon does not bind a public interface", addr)
	}
	return nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// masters is a repeatable flag, and nothing more: who holds the master ACL
// is the daemon's own configuration. See docs/02-access.md#acl.
type masters []string

func (m *masters) String() string     { return strings.Join(*m, ",") }
func (m *masters) Set(v string) error { *m = append(*m, v); return nil }

func defaultDumpFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "agent-bus", "dump.json")
}

func defaultTokenFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "agent-bus", "token")
}
