// agent-bus-setup installs the separate-user arrangement: two accounts that
// own nothing but the bus, their homes under /var/lib, and a unit that starts
// the daemon as one of them. It is the one program that wants root, it wants
// it once, and nothing after it does — the daemon never has it.
// See docs/09-setup.md#the-programs.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/parf/ai-agent-bus/internal/api"
	"github.com/parf/ai-agent-bus/internal/protocol"
	"github.com/parf/ai-agent-bus/internal/version"
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
	if version.Print() {
		return
	}
	if err := setup(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setup() (err error) {
	fs := flag.CommandLine
	owner := fs.String("owner", defaultInstaller(), "the principal the daemon belongs to: a `name`, the installer's account name by default")
	addr := fs.String("addr", "127.0.0.1:6767", "the daemon's loopback `address`")
	exe := fs.String("exec", "", "source-build or package-less acceptance `path` to agent-busd; bypasses package installation")
	keyF := fs.String("key", "", "the installer's public `key`, to be the first user; defaults to their id_ed25519.pub")
	printUnit := fs.Bool("print-unit", false, "write the unit to stdout and change nothing")
	dry := fs.Bool("dry-run", false, "say what would be done and change nothing")
	upgrade := fs.Bool("upgrade", false, "upgrade an existing packaged installation, preserving configuration and state")
	recover := fs.Bool("recover", false, "roll back an interrupted packaged upgrade")
	reinstall := fs.Bool("reinstall", false, "stop the daemon, set its whole state aside, and install fresh: the 0.7 cutover")
	samples := fs.Bool("samples", false, "add sample users, agents, services, queues, topics and groups (Star Wars and Spaceballs) to the running node")
	removeSamplesF := fs.Bool("remove-samples", false, "take the sample data away again: unregister its records, empty its groups, deactivate its users")
	var users list
	fs.Var(&users, "user", "a local account and the principal it is: `account=user[@realm]`; repeatable")
	fs.Parse(os.Args[1:])
	if *samples || *removeSamplesF {
		if *samples && *removeSamplesF {
			return fmt.Errorf("choose one of --samples or --remove-samples")
		}
		if fs.NFlag() != 1 {
			return fmt.Errorf("--samples and --remove-samples take no other option")
		}
		if _, root := ownerSocket(); root && os.Geteuid() != 0 {
			return fmt.Errorf("the samples are written as the daemon Owner, on the daemon account's socket; run sudo %s", strings.Join(os.Args, " "))
		}
		if *samples {
			return addSamples()
		}
		return removeSamples()
	}
	if *reinstall && (*upgrade || *recover) {
		return fmt.Errorf("--reinstall cannot be combined with --upgrade or --recover")
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
			return fmt.Errorf("%s needs root; run sudo %s", map[bool]string{true: "--upgrade", false: "--recover"}[*upgrade], strings.Join(os.Args, " "))
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
	// for. See docs/09-setup.md#local-users.
	if who := invoker(); who != "" {
		users = append(list{who + "=" + me.String()}, users...)
	}
	// And so does the runner: reaching the local bus over the socket makes it
	// a mapped local account like every other, which is the whole claim of
	// the split made concrete.
	// See docs/09-setup.md#the-two-units.
	users = append(users, runAccount+"="+runAccount)
	explicitExe := *exe != ""
	var selectRelease func() error
	if !explicitExe {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		if *printUnit || *dry || os.Geteuid() != 0 {
			*exe = filepath.Join(filepath.Dir(self), "agent-busd")
		} else if *reinstall {
			bundle, err := checkBundle(filepath.Dir(self))
			if err != nil {
				return err
			}
			// A reinstall replaces whichever release is installed, so the
			// "already installed" refusal is not asked. The release is staged
			// and verified now but selected only once the old daemon has
			// stopped and its state is set aside.
			if _, err := stageBundle(bundle); err != nil {
				return fmt.Errorf("install package: %w", err)
			}
			*exe = filepath.Join(installRoot, "current", "agent-busd")
			selectRelease = func() error {
				if err := installCommandLinks(); err != nil {
					return err
				}
				_, err := selectBundle(bundle.releaseID)
				return err
			}
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
	unit := unitFor(*exe, *addr, me.String(), users)
	if *printUnit {
		fmt.Print(unit)
		return nil
	}
	if *keyF == "" {
		*keyF = installerKey()
	}
	aside := fmt.Sprintf("%s.before-0.7-%s-%d", svcHome, time.Now().Format("20060102-150405.000000"), os.Getpid())
	steps := []string{}
	if *reinstall {
		steps = append(steps, "stop agent-busd; a node that will not stop, or a daemon holding its home outside systemd, is not reinstalled",
			fmt.Sprintf("set aside everything in %s, and the unit's drop-ins, in the root-only %s; nothing there is read again unless a later step fails, which puts it all back", svcHome, aside))
	}
	steps = append(steps,
		fmt.Sprintf("create the system account %s with home %s", svcAccount, svcHome),
		fmt.Sprintf("create the system account %s with home %s", runAccount, runHome),
	)
	for _, d := range dirs {
		steps = append(steps, fmt.Sprintf("make %s %s's own, %#o", d.path, d.owner, d.mode))
	}
	if !explicitExe {
		steps = append([]string{"install the complete release under " + installRoot}, steps...)
	}
	steps = append(steps,
		fmt.Sprintf("make %s the daemon's to write and adm's to read, rotated by %s", logDir, logrotate),
		fmt.Sprintf("initialize the database %s as %s", dbPath, svcAccount),
		fmt.Sprintf("write %s", unitPath),
		"reload systemd and start agent-busd, restarting a running one whose unit or release changed, and wait until it reports the installed release")
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
		return fmt.Errorf("this needs root once, to %s and %s. Run:\n\n    sudo %s\n\n"+
			"Nothing after this step does: the daemon runs as %s.\n"+
			"Use --dry-run to see the steps, or --print-unit for the unit alone",
			steps[0], unitPath, strings.Join(os.Args, " "), svcAccount)
	}
	// What the node was before this run: a reinstall that fails midway puts it
	// back, and setup over a running node restarts it only when that is what
	// applies the change.
	priorUnit, unitErr := os.ReadFile(unitPath)
	if unitErr != nil && !errors.Is(unitErr, os.ErrNotExist) {
		return unitErr
	}
	hadUnit := unitErr == nil
	priorRelease, releaseErr := priorReleaseForRollback()
	if releaseErr != nil {
		return releaseErr
	}
	if *reinstall {
		// Not `:=` on err: the rollback below reads setup's own result.
		wasActive, asideErr := setAside(aside)
		if asideErr != nil {
			return asideErr
		}
		defer func() {
			if err != nil {
				err = rollbackReinstall(aside, priorUnit, hadUnit, priorRelease, wasActive, err)
			}
		}()
	}
	active := exec.Command("systemctl", "is-active", "--quiet", "agent-busd").Run() == nil
	if selectRelease != nil {
		if err := selectRelease(); err != nil {
			return fmt.Errorf("install package: %w", err)
		}
	}
	// sshd executes even a forced command through the account's shell. The
	// daemon account therefore needs sh; restrict,command= on every issued key
	// supplies the SSH boundary. The runner has no SSH entry point.
	// See docs/09-setup.md#the-two-accounts.
	for _, a := range []struct{ name, home, shell string }{
		{svcAccount, svcHome, "/bin/sh"},
		{runAccount, runHome, "/usr/sbin/nologin"},
	} {
		if _, err := user.Lookup(a.name); err == nil {
			if a.name == svcAccount {
				// Repair installations created with the old nologin shell too.
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
	// A host without logrotate installed has no logrotate.d; the rule is
	// still written, so installing logrotate later rotates these logs.
	if err := os.MkdirAll(filepath.Dir(logrotate), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(logrotate, []byte(logrotateConf), 0o644); err != nil {
		return err
	}
	// The database is made here, explicitly, as the daemon's account: the unit
	// never creates one, so a database lost later refuses the start rather
	// than silently becoming an empty node (Plans/MVP/0.7-cutover.md#procedure).
	// An existing database is the node's state: setup over a running node,
	// an identical reinstall included, uses it rather than initializing it
	// again, which the daemon's exclusive lock would refuse anyway.
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		if err := run("runuser", "-u", svcAccount, "--", *exe, "-owner", me.String(), "-db", dbPath, "-init"); err != nil {
			return err
		}
	} else if err != nil {
		return err
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
	// unit or a replaced binary is applied by a restart, and setup waits, as
	// --upgrade does, until the daemon reports the release it installed.
	wantVersion, wantBuild, err := programVersion(*exe)
	if err != nil {
		return err
	}
	start := "start"
	if active && (!bytes.Equal(priorUnit, []byte(unit)) || !hadUnit ||
		!serving(*addr, wantVersion, wantBuild)) {
		start = "restart"
	}
	if err := run("systemctl", start, "agent-busd"); err != nil {
		return err
	}
	node, err := waitNodeIdentity(*addr, wantVersion, wantBuild, 20*time.Second)
	if err != nil {
		return err
	}
	// The web face is its own unit; a node without it still serves every
	// other face, so a failure is reported and not fatal.
	if err := installWeb(webDirFor(*exe)); err != nil {
		fmt.Fprintf(os.Stderr, "web: %v; agent-busd serves without its web face\n", err)
	}
	// The first user is the installer, and adding one is the admin program's
	// job — setup does not learn a second way to do it.
	// See docs/09-setup.md#the-programs.
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
	fmt.Printf("agent-busd %s (%s) runs as %s, owned by %s, state in %s; services are %s's, in %s\n",
		node.Version, node.Build, svcAccount, node.Owner, svcHome, runAccount, svcDir)
	return nil
}

// setAside is the reinstall's first half (Plans/MVP/0.7-cutover.md#procedure):
// the daemon is stopped, and everything it kept — the database or the 0.6
// dump, the credentials, authorized keys — and every drop-in that would
// override the new unit moves into one root-only directory beside its home.
// Nothing there is read again. A daemon that will not stop, or one still
// holding its home outside systemd, is not reinstalled, and an existing
// set-aside directory is never overwritten. It reports whether the unit was
// running, so a failure later puts the node back as it was.
func setAside(aside string) (bool, error) {
	// A host that never had the unit has nothing to stop: that is the fresh
	// case, not a daemon that refuses to stop.
	wasActive := exec.Command("systemctl", "is-active", "--quiet", "agent-busd").Run() == nil
	load, _ := exec.Command("systemctl", "show", "-p", "LoadState", "--value", "agent-busd").Output()
	if strings.TrimSpace(string(load)) != "not-found" {
		if err := run("systemctl", "stop", "agent-busd"); err != nil {
			return false, fmt.Errorf("agent-busd will not stop, so it is not reinstalled: %w", err)
		}
	}
	if out, _ := exec.Command("systemctl", "is-active", "agent-busd").Output(); strings.TrimSpace(string(out)) == "active" {
		return false, fmt.Errorf("agent-busd is still active, so it is not reinstalled")
	}
	// A daemon started by hand holds the database outside the unit, and
	// moving the file from under it would leave two nodes.
	pids, err := holders(svcHome)
	if err == nil && len(pids) != 0 {
		err = fmt.Errorf("process %s still holds %s outside systemd, so it is not reinstalled; stop it and run setup again",
			strings.Join(pids, ", "), svcHome)
	}
	if err == nil {
		err = moveAside(aside, svcHome, unitPath+".d")
	}
	if err != nil {
		if wasActive {
			if startErr := startDaemon(); startErr != nil {
				return false, fmt.Errorf("%w; restarting agent-busd also failed: %v", err, startErr)
			}
		}
		return false, err
	}
	fmt.Printf("set aside the old daemon state in %s\n", aside)
	return wasActive, nil
}

// moveAside is setAside's file half: home's entries go to aside/home and the
// drop-ins to aside/agent-busd.service.d, inside a new directory only root
// can open.
func moveAside(aside, home, dropins string) error {
	if err := os.Mkdir(aside, 0o700); err != nil {
		return fmt.Errorf("set aside: %w", err)
	}
	// Mkdir honours the umask; the mode is the design, so it is set.
	if err := os.Chmod(aside, 0o700); err != nil {
		return fmt.Errorf("set aside: %w", err)
	}
	if err := moveEntries(home, filepath.Join(aside, "home")); err != nil {
		return fmt.Errorf("set aside %s: %w", home, err)
	}
	if err := moveEntries(dropins, filepath.Join(aside, "agent-busd.service.d")); err != nil {
		return fmt.Errorf("set aside the unit's drop-ins: %w", err)
	}
	return nil
}

// moveEntries moves every entry of from into into, creating into 0700 only
// when there is something to move.
func moveEntries(from, into string) error {
	entries, err := os.ReadDir(from)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	if err := os.MkdirAll(into, 0o700); err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.Rename(filepath.Join(from, e.Name()), filepath.Join(into, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// restoreAside undoes moveAside. What the failed install left in home is
// kept, root-only, in aside/failed-reinstall rather than deleted; aside is
// removed when nothing remains in it.
func restoreAside(aside, home, dropins string) error {
	if err := moveEntries(home, filepath.Join(aside, "failed-reinstall")); err != nil {
		return err
	}
	if err := moveEntries(filepath.Join(aside, "home"), home); err != nil {
		return err
	}
	if err := moveEntries(filepath.Join(aside, "agent-busd.service.d"), dropins); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(aside, "home"))
	_ = os.Remove(filepath.Join(aside, "agent-busd.service.d"))
	_ = os.Remove(aside)
	return nil
}

// rollbackReinstall is a reinstall that failed after its state was set
// aside: the old home, drop-ins, unit and release come back, and a node that
// was running is started again rather than left stopped.
func rollbackReinstall(aside string, priorUnit []byte, hadUnit bool, priorRelease string, wasActive bool, cause error) error {
	_ = run("systemctl", "stop", "agent-busd")
	var failed []string
	if err := restoreAside(aside, svcHome, unitPath+".d"); err != nil {
		failed = append(failed, "state: "+err.Error())
	}
	if hadUnit {
		if err := os.WriteFile(unitPath, priorUnit, 0o644); err != nil {
			failed = append(failed, "unit: "+err.Error())
		}
	} else if err := os.Remove(unitPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		failed = append(failed, "unit: "+err.Error())
	}
	if priorRelease != "" {
		if _, err := selectBundle(priorRelease); err != nil {
			failed = append(failed, "release: "+err.Error())
		}
	}
	if err := run("systemctl", "daemon-reload"); err != nil {
		failed = append(failed, err.Error())
	}
	if wasActive && len(failed) == 0 {
		if err := startDaemon(); err != nil {
			failed = append(failed, err.Error())
		}
	}
	if len(failed) != 0 {
		return fmt.Errorf("%v; rollback failed (%s); the old state is in %s", cause, strings.Join(failed, "; "), aside)
	}
	state := "left stopped, as it was"
	if wasActive {
		state = "started again"
	}
	return fmt.Errorf("%v; rolled back: the old state, unit and drop-ins are restored and agent-busd is %s", cause, state)
}

// holders are the processes with a file under dir open.
func holders(dir string) ([]string, error) {
	procs, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	prefix := filepath.Clean(dir) + "/"
	var pids []string
	for _, p := range procs {
		if p.Name()[0] < '0' || p.Name()[0] > '9' {
			continue
		}
		fdDir := filepath.Join("/proc", p.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // gone, or not ours to read
		}
		for _, fd := range fds {
			if target, err := os.Readlink(filepath.Join(fdDir, fd.Name())); err == nil && strings.HasPrefix(target, prefix) {
				pids = append(pids, p.Name())
				break
			}
		}
	}
	return pids, nil
}

// serving is one short look at whether the running daemon is already the
// installed release and build.
func serving(addr, version, build string) bool {
	_, err := waitNodeIdentity(addr, version, build, time.Second)
	return err == nil
}

// programVersion is what a program's --version reports: the release and its
// build_info stamp.
func programVersion(exe string) (string, string, error) {
	out, err := exec.Command(exe, "--version").Output()
	if err != nil {
		return "", "", fmt.Errorf("%s --version: %w", exe, err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[1], "build_info: ") {
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
ExecStart=%[3]s -addr %[4]s -socket %[6]s -db %[2]s/agent-bus.db -log-dir %[8]s -owner %[7]s`,
		svcAccount, svcHome, exe, addr, filepath.Base(api.SystemRuntimeDir), api.SystemSocket(), owner, logDir)
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
// defaults to it, with no realm appended (docs/01-identity-and-roles.md#names).
func defaultInstaller() string { return invoker() }

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
