// agent-bus-setup installs the separate-user arrangement: two accounts that
// own nothing but the bus, their homes under /var/lib, and a unit that starts
// the daemon as one of them. It is the one program that wants root, it wants
// it once, and nothing after it does — the daemon never has it.
// See docs/09-setup.md#the-programs.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/protocol"
)

// Where the install puts things. Stated here because setup is the only thing
// that writes them; the daemon is told on its command line.
// See docs/09-setup.md#the-two-accounts.
const (
	svcAccount = "agent-busd"
	runAccount = "agent-bus-runner"
	stateRoot  = "/var/lib/agent-bus"
	svcHome    = stateRoot + "/daemon"
	svcDir     = stateRoot + "/service.d"
	runHome    = stateRoot + "/runner"
	unitPath   = "/etc/systemd/system/agent-busd.service"
)

// dirs is the layout, and the modes are the design rather than a default: the
// daemon's home and the runner's are each their own account's alone, and
// service.d is world-readable because what a service *is* holds no secret.
// See docs/09-setup.md#the-two-accounts.
var dirs = []struct {
	path  string
	owner string
	mode  os.FileMode
}{
	{svcHome, svcAccount, 0o700},
	{svcDir, runAccount, 0o755},
	{runHome, runAccount, 0o700},
}

type list []string

func (l *list) String() string     { return strings.Join(*l, ",") }
func (l *list) Set(v string) error { *l = append(*l, v); return nil }

func main() {
	if err := setup(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setup() error {
	fs := flag.CommandLine
	owner := fs.String("owner", defaultInstaller(), "the principal the daemon belongs to: `user@realm`")
	addr := fs.String("addr", "127.0.0.1:7777", "the daemon's loopback `address`")
	exe := fs.String("exec", "", "`path` to agent-busd; defaults to the one beside this binary")
	keyF := fs.String("key", "", "the installer's public `key`, to be the first user; defaults to their id_ed25519.pub")
	printUnit := fs.Bool("print-unit", false, "write the unit to stdout and change nothing")
	dry := fs.Bool("dry-run", false, "say what would be done and change nothing")
	var users list
	fs.Var(&users, "user", "a local account and the principal it is: `account=user@realm`; repeatable")
	fs.Parse(os.Args[1:])
	me, err := protocol.ParseName(*owner)
	if err != nil {
		return fmt.Errorf("--owner: %w", err)
	}
	// The installer is a user of the bus like anyone else, and the one who
	// will hold admin — so their account gets a socket without being asked
	// for. See docs/09-setup.md#local-users.
	if who := invoker(); who != "" {
		users = append(list{who + "=" + me.String()}, users...)
	}
	// And so does the runner: reaching the local bus over the socket makes it
	// a mapped local account like every other, which is the whole claim of
	// the split made concrete.
	// See docs/09-setup.md#the-two-units.
	users = append(users, runAccount+"=runner@"+me.Realm)
	if *exe == "" {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		*exe = filepath.Join(filepath.Dir(self), "agent-busd")
	}
	unit := unitFor(*exe, *addr, me.String(), users)
	if *printUnit {
		fmt.Print(unit)
		return nil
	}
	if *keyF == "" {
		*keyF = installerKey()
	}
	steps := []string{
		fmt.Sprintf("create the system account %s with home %s", svcAccount, svcHome),
		fmt.Sprintf("create the system account %s with home %s", runAccount, runHome),
	}
	for _, d := range dirs {
		steps = append(steps, fmt.Sprintf("make %s %s's own, %#o", d.path, d.owner, d.mode))
	}
	steps = append(steps,
		fmt.Sprintf("write %s", unitPath),
		"reload systemd and start agent-busd")
	if *keyF != "" {
		steps = append(steps, fmt.Sprintf("make %s the first user, from %s", me, *keyF))
	}
	if *dry {
		for _, s := range steps {
			fmt.Println("would " + s)
		}
		return nil
	}
	// Refused rather than half-done: the account and the unit are both root's
	// to write, and a setup that created neither is easier to recover from
	// than one that created one.
	if os.Geteuid() != 0 {
		return fmt.Errorf("this needs root once, to %s and %s. Run:\n\n    sudo %s\n\n"+
			"Nothing after this step does: the daemon runs as %s.\n"+
			"Use --dry-run to see the steps, or --print-unit for the unit alone",
			steps[0], unitPath, strings.Join(os.Args, " "), svcAccount)
	}
	// Two accounts, because there are two secret domains and neither may read
	// the other's. Both nologin: neither is one you log in as.
	// See docs/09-setup.md#the-two-accounts.
	for _, a := range []struct{ name, home string }{{svcAccount, svcHome}, {runAccount, runHome}} {
		if _, err := user.Lookup(a.name); err == nil {
			continue
		}
		if err := run("useradd", "--system", "--home-dir", a.home, "--create-home",
			"--shell", "/usr/sbin/nologin", a.name); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(stateRoot, 0o755); err != nil {
		return err
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d.path, d.mode); err != nil {
			return err
		}
		if err := run("chown", d.owner+":"+d.owner, d.path); err != nil {
			return err
		}
		// MkdirAll honours the umask and skips a directory that already
		// exists, so the mode is set rather than asked for.
		if err := os.Chmod(d.path, d.mode); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return err
	}
	if err := run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := run("systemctl", "enable", "--now", "agent-busd"); err != nil {
		return err
	}
	// The first user is the installer, and adding one is the admin program's
	// job — setup does not learn a second way to do it.
	// See docs/09-setup.md#the-programs.
	if *keyF != "" {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		// The key is read here, by root, and handed over on stdin: the admin
		// program runs as agent-busd, and a key sitting in a person's 0700
		// home is precisely what that account cannot open.
		key, err := os.ReadFile(*keyF)
		if err != nil {
			return err
		}
		if err := runIn(key, filepath.Join(filepath.Dir(self), "agent-bus-admin"),
			"user", "add", me.String(), "-", "--admin"); err != nil {
			return err
		}
	}
	fmt.Printf("agent-busd runs as %s, owned by %s, state in %s; services are %s's, in %s\n",
		svcAccount, me, svcHome, runAccount, svcDir)
	return nil
}

// installerKey is the public half the person running this already has. A
// missing one is not a failure: they can add it later, and the step says so.
func installerKey() string {
	who := invoker()
	if who == "" {
		return ""
	}
	u, err := user.Lookup(who)
	if err != nil || u.HomeDir == "" {
		return ""
	}
	for _, k := range []string{"id_ed25519.pub", "id_ecdsa.pub", "id_rsa.pub"} {
		p := filepath.Join(u.HomeDir, ".ssh", k)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// unitFor is the unit, and the only place its values are written down.
// See docs/09-setup.md#the-two-accounts.
func unitFor(exe, addr, owner string, users list) string {
	var b strings.Builder
	fmt.Fprintf(&b, `[Unit]
Description=agent-bus: registry, broker and MCP server for agents
After=network.target

[Service]
Type=simple
# The account, never root and never whoever ran setup.
User=%[1]s
Group=%[1]s
WorkingDirectory=%[2]s
StateDirectory=agent-bus/daemon
# 0700, stated: systemd re-applies its own default to a StateDirectory on
# every start, so leaving it out silently widens the home setup made 0700.
StateDirectoryMode=0700
RuntimeDirectory=%[5]s
# 0711: everyone walks through to their own socket, nobody reads the rest.
RuntimeDirectoryMode=0711
ExecStart=%[3]s -addr %[4]s -socket %[6]s -token-file %[2]s/token -dump-file %[2]s/dump.json -owner %[7]s`,
		svcAccount, svcHome, exe, addr, filepath.Base(api.SystemRuntimeDir), api.SystemSocket(), owner)
	for _, u := range users {
		fmt.Fprintf(&b, " -user %s", u)
	}
	fmt.Fprintf(&b, `
Restart=on-failure
RestartSec=2
# One capability, declared rather than taken: a per-account socket has to be
# handed to its account. The supervisor keeps it and no child inherits it —
# docs/11-processes.md#why-the-supervisor-holds-cap_chown
AmbientCapabilities=CAP_CHOWN
CapabilityBoundingSet=CAP_CHOWN
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=%s

[Install]
WantedBy=multi-user.target
`, svcHome)
	return b.String()
}

// invoker is the account that asked for the install, which is not the one
// running the command when that is sudo.
func invoker() string {
	if u := os.Getenv("SUDO_USER"); u != "" {
		return u
	}
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return ""
}

func defaultInstaller() string {
	who := invoker()
	if who == "" {
		return ""
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

func run(name string, args ...string) error {
	return runIn(nil, name, args...)
}

func runIn(stdin []byte, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
