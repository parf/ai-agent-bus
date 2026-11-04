// agent-bus-setup installs the separate-user arrangement: two accounts that
// own nothing but the bus, their homes under /var/lib, and a unit that starts
// the daemon as one of them. It is the one program that wants root, it wants
// it once, and nothing after it does — the daemon never has it.
// See docs/07-setup.md#the-programs.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/backup"
	"github.com/parf/ai-agent-bus/internal/display"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/tlsdir"
	"github.com/parf/ai-agent-bus/internal/version"
)

// Where the install puts things. Stated here because setup is the only thing
// that writes them; the daemon is told on its command line.
// See docs/07-setup.md#the-two-accounts.
const (
	svcAccount = "agent-busd"
	stateRoot  = "/var/lib/agent-bus"
	svcHome    = stateRoot + "/daemon"
	unitPath   = "/etc/systemd/system/agent-busd.service"
	dbPath     = svcHome + "/agent-bus.db"
	logDir     = "/var/log/agent-bus"
	logrotate  = "/etc/logrotate.d/agent-bus"
)

// logrotateConf rotates the three logs by copying and truncating, so the
// daemon never has to reopen a file (docs/constitution.md#logs).
const logrotateConf = `/var/log/agent-bus/*.log {
    weekly
    rotate 8
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
    su agent-busd adm
}
`

// installLogrotate writes the rule at path. A host without logrotate installed
// has no logrotate.d; the rule is still written, so installing logrotate later
// rotates these logs.
func installLogrotate(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(logrotateConf), 0o644)
}

// dirs is the layout, and the mode is the design rather than a default: the
// daemon's home is its account's alone. A person's runner has its own
// (runner_install.go). See docs/07-setup.md#the-two-accounts.
var dirs = []struct {
	path  string
	owner string
	mode  os.FileMode
}{
	{svcHome, svcAccount, 0o700},
	{backup.DefaultDirectory, svcAccount, 0o700},
}

type list []string

func (l *list) String() string     { return strings.Join(*l, ",") }
func (l *list) Set(v string) error { *l = append(*l, v); return nil }

func main() {
	if version.Print() {
		return
	}
	// A user-account runner's work under a person's home, as that person.
	if len(os.Args) > 1 && os.Args[1] == helperFlag {
		if err := userHelper(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := setup(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setup() (err error) {
	fs := flag.CommandLine
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "agent-bus-setup %s — install, upgrade and maintain this node\n\nUsage of agent-bus-setup:\n", version.String)
		display.PrintFlagDefaults(fs)
		fmt.Fprintln(fs.Output(), "\n"+display.Footer())
	}
	owner := fs.String("owner", defaultInstaller(), "the principal the daemon belongs to: a `name`, the installer's account name by default")
	addr := fs.String("addr", "127.0.0.1:6767", "the daemon's TCP `address`; any interface, plain HTTP")
	exe := fs.String("exec", "", "source-build or package-less acceptance `path` to agent-busd; bypasses package installation")
	keyF := fs.String("key", "", "the installer's public `key`, to be the first user; defaults to their id_ed25519.pub")
	printUnit := fs.Bool("print-unit", false, "write the unit to stdout and change nothing")
	dry := fs.Bool("dry-run", false, "say what would be done and change nothing")
	upgrade := fs.Bool("upgrade", false, "upgrade an existing packaged installation, preserving configuration and state")
	recover := fs.Bool("recover", false, "roll back an interrupted packaged upgrade")
	samples := fs.Bool("samples", false, "add sample users, agents, services, queues, pubsub channels and groups (Star Wars and Spaceballs) to the running node")
	removeSamplesF := fs.Bool("remove-samples", false, "take the sample data away again: unregister its records, empty its groups, deactivate its users")
	runnerV := fs.String("runner", "", "a person's runner: `install`, uninstall, enable or disable (docs/07-setup.md#a-persons-runner)")
	runnerFor := fs.String("for", invoker(), "whose runner, a bus User's plain `name`; the installer by default")
	purge := fs.Bool("purge", false, "with --runner uninstall, remove private state (shared agent.d is always kept)")
	userRunnerF := fs.Bool("user-runner", false, "with --runner: the person's user-account runner under their own user manager (docs/07-setup.md#a-persons-user-account-runner); also verb refresh")
	tlsC := tlsFlags(fs)
	var users list
	fs.Var(&users, "user", "a local account and the principal it is: `account=user[@realm]`; repeatable")
	// Asked for help, it answers on stdout and succeeds, as every agent-bus
	// program does; the flag package would say it on stderr and exit 2. It
	// answers only once every flag is defined, so the list names them all.
	if len(os.Args) == 2 && os.Args[1] == "--help" {
		fs.SetOutput(os.Stdout)
		fs.Usage()
		os.Exit(0)
	}
	if err := display.ParseFlags(fs, os.Args[1:]); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		return darwinSetup(fs, darwinInstall, *owner, *addr, *exe, *keyF, *printUnit, *dry, tlsC)
	}
	if *runnerV != "" {
		var contrary []string
		fs.Visit(func(f *flag.Flag) {
			if f.Name != "runner" && f.Name != "for" && f.Name != "purge" && f.Name != "dry-run" && f.Name != "user-runner" {
				contrary = append(contrary, "--"+f.Name)
			}
		})
		if len(contrary) != 0 {
			return fmt.Errorf("--runner takes --for, --purge, --user-runner and --dry-run, not %s", strings.Join(contrary, ", "))
		}
		if *userRunnerF {
			return userRunnerVerb(*runnerV, *runnerFor, *purge, *dry)
		}
		return runnerVerb(*runnerV, *runnerFor, *purge, *dry)
	}
	if *purge {
		return fmt.Errorf("--purge goes with --runner uninstall")
	}
	if *samples || *removeSamplesF {
		if *samples && *removeSamplesF {
			return fmt.Errorf("choose one of --samples or --remove-samples")
		}
		if fs.NFlag() != 1 {
			return fmt.Errorf("--samples and --remove-samples take no other option")
		}
		if _, root := ownerSocket(); root && os.Geteuid() != 0 {
			return fmt.Errorf("the samples are written as the daemon Owner, on the daemon account's socket; run %s", sudoLine())
		}
		if *samples {
			return addSamples()
		}
		return removeSamples()
	}
	if err := tlsC.check(); err != nil {
		return err
	}
	if tlsC.mode != "" && (*upgrade || *recover) {
		return fmt.Errorf("--tls is set by setup; --upgrade keeps the node's TLS as it is")
	}
	if *upgrade || *recover {
		if *upgrade && *recover {
			return fmt.Errorf("choose one of --upgrade or --recover")
		}
		var contrary []string
		fs.Visit(func(f *flag.Flag) {
			if f.Name != "upgrade" && f.Name != "recover" {
				contrary = append(contrary, "--"+f.Name)
			}
		})
		if len(contrary) != 0 {
			return fmt.Errorf("%s cannot be combined with %s", map[bool]string{true: "--upgrade", false: "--recover"}[*upgrade], strings.Join(contrary, ", "))
		}
		if os.Geteuid() != 0 {
			return fmt.Errorf("%s needs root; run %s", map[bool]string{true: "--upgrade", false: "--recover"}[*upgrade], sudoLine())
		}
		if *recover {
			return recoverUpgrade()
		}
		self, err := os.Executable()
		if err != nil {
			return err
		}
		bundle, err := checkBundle(filepath.Dir(self))
		if err != nil {
			return err
		}
		return upgradeBundle(bundle)
	}
	me, err := protocol.ParseName(*owner)
	if err != nil {
		return fmt.Errorf("--owner: %w", err)
	}
	// The installer is a user of the bus like anyone else, and the one who
	// will hold admin — so their account gets a socket without being asked
	// for. See docs/07-setup.md#local-users.
	if who := invoker(); who != "" {
		users = append(list{who + "=" + me.String()}, users...)
	}
	explicitExe := *exe != ""
	if !explicitExe {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		if *printUnit || *dry || os.Geteuid() != 0 {
			*exe = filepath.Join(filepath.Dir(self), "agent-busd")

		} else {
			bundle, err := checkBundle(filepath.Dir(self))
			if err != nil {
				return err
			}
			installed, err := installBundle(bundle)
			if err != nil {
				return fmt.Errorf("install package: %w", err)
			}
			*exe = filepath.Join(installed, "agent-busd")
		}
	}
	// TLS on the daemon's port: asked at a terminal, otherwise what the flags
	// say, otherwise what the node already has (docs/07-setup.md#tls).
	if tlsC.mode == "" && !*printUnit && !*dry && !*upgrade && !*recover && terminal() {
		if err := tlsC.ask(os.Stdin, os.Stdout); err != nil {
			return err
		}
	}
	if tlsC.enabled() && tlsC.mode == "" {
		if _, err := tlsdir.Load(tlsC.dir); err != nil {
			return fmt.Errorf("the node's TLS directory %s does not load, and the daemon would refuse to start: %w; fix it, or pass --tls off", tlsC.dir, err)
		}
	}
	tlsDirArg := ""
	if tlsC.enabled() {
		tlsDirArg = tlsC.dir
	}
	unit := unitFor(*exe, *addr, me.String(), users, tlsDirArg)
	if *printUnit {
		fmt.Print(unit)
		return nil
	}
	if *keyF == "" {
		*keyF = installerKey()
	}
	steps := []string{}
	steps = append(steps,
		fmt.Sprintf("create the system account %s with home %s", svcAccount, svcHome),
	)
	for _, d := range dirs {
		steps = append(steps, fmt.Sprintf("make %s %s's own, %#o", d.path, d.owner, d.mode))
	}
	if !explicitExe {
		steps = append([]string{"install the complete release under " + installRoot}, steps...)
	}
	steps = append(steps,
		fmt.Sprintf("make %s the daemon's to write and adm's to read, rotated by %s", logDir, logrotate),
		fmt.Sprintf("initialize the database %s as %s", dbPath, svcAccount))
	if s := tlsC.step(); s != "" {
		steps = append(steps, s)
	}
	steps = append(steps,
		fmt.Sprintf("write %s", unitPath),
		"reload systemd and start agent-busd, restarting a running one whose unit, release or TLS changed, and wait until it reports the installed release")
	steps = append(steps, webSteps(webDirFor(*exe))...)
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
		return fmt.Errorf("this needs root once, to %s and %s. Run:\n\n    %s\n\n"+
			"Nothing after this step does: the daemon runs as %s.\n"+
			"Use --dry-run to see the steps, or --print-unit for the unit alone",
			steps[0], unitPath, sudoLine(), svcAccount)
	}
	// A current-format setup restarts a running node only when its inputs change.
	priorUnit, unitErr := os.ReadFile(unitPath)
	if unitErr != nil && !errors.Is(unitErr, os.ErrNotExist) {
		return unitErr
	}
	hadUnit := unitErr == nil
	active := exec.Command("systemctl", "is-active", "--quiet", "agent-busd").Run() == nil
	// sshd executes even a forced command through the account's shell. The
	// daemon account therefore needs sh; restrict,command= on every issued key
	// supplies the SSH boundary. The runner has no SSH entry point.
	// See docs/07-setup.md#the-two-accounts.
	for _, a := range []struct{ name, home, shell string }{
		{svcAccount, svcHome, "/bin/sh"},
	} {
		if _, err := user.Lookup(a.name); err == nil {
			if a.name == svcAccount {
				// Keep the current account shell consistent with its SSH entry point.
				if err := repairSSHShell(a.name, a.shell); err != nil {
					return err
				}
			}
			continue
		}
		if err := run("useradd", "--system", "--home-dir", a.home, "--create-home",
			"--shell", a.shell, a.name); err != nil {
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
	if err := recordBackupDirectory(); err != nil {
		return err
	}
	// The logs are the daemon's to write and adm's to read: set-group-ID to
	// adm, so every file the daemon creates there is adm's group too.
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return err
	}
	if err := run("chown", svcAccount+":adm", logDir); err != nil {
		return err
	}
	if err := os.Chmod(logDir, 0o750|os.ModeSetgid); err != nil {
		return err
	}
	if err := installLogrotate(logrotate); err != nil {
		return err
	}
	// The database is made here, explicitly, as the daemon's account: the unit
	// never creates one, so a database lost later refuses the start rather
	// than silently becoming an empty node (Plans/done/R0.8-MVP/0.7-cutover.md#procedure).
	// An existing database is the node's state: setup over a running node,
	// including an identical rerun, uses it rather than initializing it
	// again, which the daemon's exclusive lock would refuse anyway.
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		if err := run("runuser", "-u", svcAccount, "--", *exe, "--owner", me.String(), "--db", dbPath, "--init"); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if _, err := tlsC.install(svcAccount, os.Stderr); err != nil {
		return fmt.Errorf("TLS: %w", err)
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
	if err := run("systemctl", "enable", "agent-busd"); err != nil {
		return err
	}
	// A running node keeps its process across `enable --now`, so a changed
	// unit, binary or TLS files are applied by a restart, and setup waits, as
	// --upgrade does, until the daemon reports the release it installed.
	wantVersion, wantBuild, err := programVersion(*exe)
	if err != nil {
		return err
	}
	if err := tlsC.startDaemon(active, !bytes.Equal(priorUnit, []byte(unit)) || !hadUnit,
		*addr, wantVersion, wantBuild); err != nil {
		return err
	}
	node, err := waitNodeIdentity(*addr, wantVersion, wantBuild, 20*time.Second)
	if err != nil {
		return err
	}
	// The web face is its own unit; a node without it still serves every
	// other face, so a failure is reported and not fatal.
	if err := installWeb(webDirFor(*exe), tlsC.enabled(), tlsC.dir); err != nil {
		fmt.Fprintf(os.Stderr, "web: %v; agent-busd serves without its web face\n", err)
	}
	// The first user is the installer, and adding one is the admin program's
	// job — setup does not learn a second way to do it.
	// See docs/07-setup.md#the-programs.
	if *keyF != "" {
		// systemctl reports the supervisor active before it has necessarily
		// created and handed the account listeners to the bus child. The first
		// administrative call must use the daemon account's credential socket,
		// never race into an anonymous fallback.
		if err := waitForSocket(api.UserSocket(api.SystemRuntimeDir, svcAccount), 10*time.Second); err != nil {
			return err
		}
		admin := filepath.Join(filepath.Dir(*exe), "agent-bus-admin")
		// The key is read here, by root, and handed over on stdin: the admin
		// program runs as agent-busd, and a key sitting in a person's 0700
		// home is precisely what that account cannot open.
		key, err := os.ReadFile(*keyF)
		if err != nil {
			return err
		}
		if err := runIn(key, admin,
			"user", "add", me.String(), "-", "--admin"); err != nil {
			return err
		}
		// PersonName comes from the account database, not from setup's caller.
		// The admin program owns that adapter and preserves an existing explicit
		// profile; setup only identifies which local account belongs to the first
		// key-backed user. With no key this onboarding step is skipped; the same
		// import-local verb remains available to the operator later.
		if who := invoker(); who != "" {
			if err := run(admin,
				"user", "import-local", me.String(), who); err != nil {
				return err
			}
		}
	}
	// The Owner is the node's durable one, which --owner seeds only once: a
	// transfer since then is what the node answers to.
	fmt.Printf("agent-busd %s (%s) runs as %s, owned by %s, state in %s; a person's runner is --runner install --for <person>\n",
		node.Version, node.Build, svcAccount, node.Owner, svcHome)
	// What a client elsewhere sets to reach the port over TLS; the same
	// fingerprint is on offer as `ssh agent-busd@<node> token --fingerprint`.
	if tlsC.enabled() {
		if cert, err := tlsdir.LoadCert(tlsC.dir); err == nil {
			fmt.Printf("TLS is on for %s beside plain HTTP, certificate %s. A client elsewhere sets:\n    AGENT_BUS_ADDR=https://<this host>:%s AGENT_BUS_TLS_FINGERPRINT=%s\n",
				*addr, tlsdir.Fingerprint(cert), portOf(*addr), tlsdir.Fingerprint(cert))
		}
	}
	return nil
}

// portOf is addr's port, or addr itself when it has none.
func portOf(addr string) string {
	if _, p, err := net.SplitHostPort(addr); err == nil {
		return p
	}
	return addr
}

func serving(addr, version, build string) bool {
	_, err := waitNodeIdentity(addr, version, build, time.Second)
	return err == nil
}

// startDaemon applies a changed unit, release or TLS file to a running node.
// A version match alone cannot detect a rotated certificate.
func (c *tlsChoice) startDaemon(active, unitChanged bool, addr, wantVersion, wantBuild string) error {
	verb := "start"
	if active && (c.written || unitChanged || !serving(addr, wantVersion, wantBuild)) {
		verb = "restart"
	}
	if err := run("systemctl", verb, "agent-busd"); err != nil {
		return startLimitHint(err)
	}
	return nil
}

// startLimitHint says what systemd's own refusal means when a start failed
// because the unit was started too often: wait for the window and run setup
// again. Any other failure is returned as it came.
func startLimitHint(err error) error {
	out, _ := exec.Command("systemctl", "show", "-p", "Result", "-p", "StartLimitIntervalUSec", "agent-busd").Output()
	var result, interval string
	for _, line := range strings.Split(string(out), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			switch k {
			case "Result":
				result = v
			case "StartLimitIntervalUSec":
				interval = v
			}
		}
	}
	if result != "start-limit-hit" {
		return err
	}
	if wait, ok := systemdSeconds(interval); ok {
		return fmt.Errorf("%w: agent-busd is in start-limit-hit, wait %ds and retry", err, wait)
	}
	return fmt.Errorf("%w: agent-busd is in start-limit-hit, wait for its start-limit window and retry", err)
}

// systemdSeconds reads systemd's timespan text ("10s", "1min 30s", "500ms"),
// rounded up to whole seconds.
func systemdSeconds(text string) (int, bool) {
	var micros float64
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return 0, false
	}
	for _, f := range fields {
		i := 0
		for i < len(f) && (f[i] >= '0' && f[i] <= '9' || f[i] == '.') {
			i++
		}
		n, err := strconv.ParseFloat(f[:i], 64)
		if err != nil {
			return 0, false
		}
		unit := map[string]float64{"us": 1, "ms": 1e3, "s": 1e6, "min": 60e6, "h": 3600e6}[f[i:]]
		if unit == 0 {
			return 0, false
		}
		micros += n * unit
	}
	return int((micros + 999999) / 1e6), true
}

// programVersion is what a program's --version reports: the release and its
// build_info stamp.
func programVersion(exe string) (string, string, error) {
	out, err := exec.Command(exe, "--version").Output()
	if err != nil {
		return "", "", fmt.Errorf("%s --version: %w", exe, err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if (len(lines) != 2 && (len(lines) != 3 || lines[2] != version.Copyright)) || !strings.HasPrefix(lines[1], "build_info: ") {
		return "", "", fmt.Errorf("%s --version did not report a release and its build_info", exe)
	}
	return strings.TrimSpace(lines[0]), strings.TrimPrefix(lines[1], "build_info: "), nil
}

func waitForSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("daemon account socket did not become ready: %s", path)
}

// Do not overwrite an operator's custom shell when repairing the old default.
func repairSSHShell(name, shell string) error {
	entry, err := exec.Command("getent", "passwd", name).Output()
	if err != nil {
		return fmt.Errorf("read %s account shell: %w", name, err)
	}
	fields := strings.Split(strings.TrimSpace(string(entry)), ":")
	if len(fields) != 7 || fields[0] != name {
		return fmt.Errorf("invalid passwd entry for %s", name)
	}
	if fields[6] == "/usr/sbin/nologin" || fields[6] == "/sbin/nologin" {
		return run("usermod", "--shell", shell, name)
	}
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
// See docs/07-setup.md#the-two-accounts.
func unitFor(exe, addr, owner string, users list, tlsDir string) string {
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
ExecStart=%[3]s --addr %[4]s --socket %[6]s --db %[2]s/agent-bus.db --log-dir %[8]s --owner %[7]s --syslog`,
		svcAccount, svcHome, exe, addr, filepath.Base(api.SystemRuntimeDir), api.SystemSocket(), owner, logDir)
	for _, u := range users {
		fmt.Fprintf(&b, " --user %s", u)
	}
	if tlsDir != "" {
		fmt.Fprintf(&b, " --tls-dir %s", tlsDir)
	}
	// The account's own authorized_keys, so an Administrator may add a
	// User's key from the web face as agent-bus-admin adds one on the host.
	fmt.Fprintf(&b, " --ssh-keys %s --ssh-token %s", filepath.Join(svcHome, ".ssh", "authorized_keys"), filepath.Join(filepath.Dir(exe), "agent-bus-token"))
	fmt.Fprintf(&b, `
Restart=on-failure
RestartSec=2
# One capability, declared rather than taken: a per-account socket has to be
# handed to its account. The supervisor keeps it and no child inherits it —
# docs/08-processes.md#why-the-supervisor-holds-cap_chown
AmbientCapabilities=CAP_CHOWN
CapabilityBoundingSet=CAP_CHOWN
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=%s %s

[Install]
WantedBy=multi-user.target
`, svcHome, logDir)
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

// defaultInstaller is the installer's account name alone: a User's name
// defaults to it, with no realm appended (docs/01-identity-and-authority.md#names).
func defaultInstaller() string { return invoker() }

// sudoLine is this same command for root to run. sudo's secure_path can
// leave /usr/local/bin out (the RHEL family), so a bare name found on PATH is
// given by its path.
func sudoLine() string {
	args := append([]string(nil), os.Args...)
	if !strings.Contains(args[0], "/") {
		if p, err := exec.LookPath(args[0]); err == nil {
			args[0] = p
		}
	}
	return "sudo " + strings.Join(args, " ")
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
