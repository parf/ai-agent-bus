package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// A person's user-account runner (RT31, RD124): `--runner install|uninstall|
// enable|disable|refresh --user-runner [--for <person>]`. It runs as the
// person under their own systemd user manager, for the few agents that must
// act as them (tmux sessions, their logins and files), beside the system
// runner that keeps everything else under its own account
// (docs/07-setup.md#a-persons-user-account-runner).
//
// Root never opens a path under the person's home: every observation and
// change there is made by this program re-executed as the person
// (userHelper), so a link the person plants can only lead where the person
// may already go. Root itself touches the shared template in
// /etc/systemd/user, the registry, loginctl and the person's user manager.

const (
	userTemplatePath = "/etc/systemd/user/agent-bus-runner.service"
	userUnit         = "agent-bus-runner.service"
	userStateRel     = ".local/state/agent-bus/runner"
	userAgentsRel    = ".local/share/agent-bus/agent.d"
	helperFlag       = "--user-runner-helper"
)

// userRunner is one person's user-account runner.
type userRunner struct {
	person   string
	uid, gid int
	home     string
	name     string // #runner/<person>-user@<host>
}

func (u userRunner) state() string  { return filepath.Join(u.home, userStateRel) }
func (u userRunner) agents() string { return filepath.Join(u.home, userAgentsRel) }

// account is a Linux login account as the system's name service has it.
type account struct {
	uid, gid    int
	home, shell string
}

// passwd looks an account up through NSS (getent), so directory users count.
var passwd = func(name string) (account, bool, error) {
	out, err := exec.Command("getent", "passwd", name).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 2 {
			return account{}, false, nil
		}
		return account{}, false, fmt.Errorf("getent passwd %s: %w", name, err)
	}
	f := strings.Split(strings.TrimSpace(string(out)), ":")
	if len(f) != 7 || f[0] != name {
		return account{}, false, fmt.Errorf("getent passwd %s: unexpected answer", name)
	}
	uid, err1 := strconv.Atoi(f[2])
	gid, err2 := strconv.Atoi(f[3])
	if err1 != nil || err2 != nil {
		return account{}, false, fmt.Errorf("getent passwd %s: unexpected ids", name)
	}
	return account{uid: uid, gid: gid, home: f[5], shell: f[6]}, true, nil
}

// uidMin is the first ordinary account's uid, from login.defs.
var uidMin = func() int {
	f, err := os.Open("/etc/login.defs")
	if err != nil {
		return 1000
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) == 2 && fields[0] == "UID_MIN" {
			if n, err := strconv.Atoi(fields[1]); err == nil {
				return n
			}
		}
	}
	return 1000
}

// userRunnerFor checks that person names an ordinary login account. The bus
// keeps no uid for a User, so the same name is the whole binding; that is
// acceptable because only root runs the install.
func userRunnerFor(person, host string) (userRunner, error) {
	if person == "" {
		return userRunner{}, fmt.Errorf("--for: whose user-account runner (no SUDO_USER to default to)")
	}
	if !personName.MatchString(person) {
		return userRunner{}, fmt.Errorf("--for %q: a person is a plain name, a lowercase letter, then letters, digits, _ and -, at most 22 characters", person)
	}
	a, ok, err := passwd(person)
	if err != nil {
		return userRunner{}, err
	}
	if !ok {
		return userRunner{}, fmt.Errorf("--for %s: no Linux account of that name; a user-account runner runs as the person's own login", person)
	}
	switch base := filepath.Base(a.shell); {
	case a.uid == 0:
		return userRunner{}, fmt.Errorf("--for %s: refusing uid 0", person)
	case a.uid < uidMin():
		return userRunner{}, fmt.Errorf("--for %s: uid %d is a system account (below UID_MIN %d)", person, a.uid, uidMin())
	case base == "nologin" || base == "false":
		return userRunner{}, fmt.Errorf("--for %s: %s is not a login account (shell %s)", person, person, a.shell)
	case !filepath.IsAbs(a.home) || a.home == "/":
		return userRunner{}, fmt.Errorf("--for %s: home %q is not a home directory", person, a.home)
	}
	return userRunner{person: person, uid: a.uid, gid: a.gid, home: filepath.Clean(a.home),
		name: "#runner/" + person + "-user@" + host}, nil
}

// userTemplate is the one user unit every person's runner is: no sandbox
// directives, because its agents act in the person's whole context (each
// agent's own sandbox stays the control); a crash loop is bounded; a stop
// cannot hang an upgrade. Each tmux session lives in its own scope beside it,
// so stopping this unit leaves them alone.
func userTemplate(exe string) string {
	return fmt.Sprintf(`[Unit]
Description=agent-bus user-account runner
Documentation=https://github.com/parf/ai-agent-bus/blob/main/docs/07-setup.md#a-persons-user-account-runner
ConditionPathExists=%%h/%[2]s/runner.env
StartLimitIntervalSec=300
StartLimitBurst=5

[Service]
EnvironmentFile=%%h/%[2]s/runner.env
ExecStart=%[1]s --agents %%h/%[3]s --host %%h/%[2]s
Restart=on-failure
RestartSec=5
# Give the runner its normal ten-second keeper drain before the ceiling.
KillMode=mixed
TimeoutStopSec=30s
# The runner gives each agent a cgroup of its own inside this subtree.
Delegate=yes

[Install]
WantedBy=default.target
`, exe, userStateRel, userAgentsRel)
}

// userSeen is the person's side, as the helper reports it.
type userSeen struct {
	State  string `json:"state"`  // missing, dir or other
	Agents string `json:"agents"` // missing, dir or other
	Token  string `json:"token"`  // runner.env's token, "" when absent; never printed
}

// userState is the host and the bus as one verb finds them.
type userState struct {
	user        bool
	seen        userSeen
	unit        string // the template on disk
	record      bool
	recordKind  string
	recordOwner string
	tokenValid  bool
	linger      bool
	enabled     bool
	active      bool
	others      int // other persons' user-account runner records
}

// userOps is everything a verb does outside this process.
type userOps interface {
	helper(u userRunner, op string, stdin []byte, args ...string) ([]byte, error)
	writeTemplate(unit string) error
	removeTemplate() error
	enableLinger(u userRunner) error
	userctl(u userRunner, args ...string) error
	createRecord(u userRunner) error
	removeRecord(u userRunner) error
	token(u userRunner) (string, error)
}

// userCtlVerbs are the only things setup asks of a person's user manager,
// and only about this unit: never a stop or restart of user@<uid>.service,
// a terminate or kill of the user, a stop of a session scope or a re-exec.
var userCtlVerbs = map[string]bool{"daemon-reload": true, "enable": true, "disable": true,
	"is-enabled": true, "is-active": true, "try-restart": true}

// userCtlArgv builds `systemctl --user -M <person>@ <verb> [--now] [unit]`
// and refuses anything else.
func userCtlArgv(person string, args ...string) ([]string, error) {
	if len(args) == 0 || !userCtlVerbs[args[0]] {
		return nil, fmt.Errorf("refusing systemctl --user %q", args)
	}
	for _, a := range args[1:] {
		if a != "--now" && a != "--quiet" && a != userUnit {
			return nil, fmt.Errorf("refusing systemctl --user %q: only %s", args, userUnit)
		}
	}
	if args[0] != "daemon-reload" && args[len(args)-1] != userUnit {
		return nil, fmt.Errorf("refusing systemctl --user %q: only %s", args, userUnit)
	}
	return append([]string{"systemctl", "--user", "-M", person + "@"}, args...), nil
}

// helperArgv runs this program as the person: their uid, gid and groups,
// no new privileges, no inherited environment.
func helperArgv(self string, u userRunner, op string, args ...string) []string {
	return append([]string{"setpriv", "--reuid=" + strconv.Itoa(u.uid), "--regid=" + strconv.Itoa(u.gid),
		"--init-groups", "--no-new-privs", "--", self, helperFlag, op}, args...)
}

func helperEnv(u userRunner) []string {
	return []string{"HOME=" + u.home, "PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8"}
}

// userPlan is what verb needs on a host in state st; empty when nothing does.
func userPlan(verb string, u userRunner, st userState, want string, purge bool, ops userOps) ([]action, error) {
	if (verb == "install" || verb == "enable") && st.record && (st.recordKind != "agent" || st.recordOwner != u.person) {
		return nil, fmt.Errorf("%s is not %s's agent; refusing to use it as their user-account runner", u.name, u.person)
	}
	for _, d := range []struct{ what, kind string }{{u.state(), st.seen.State}, {u.agents(), st.seen.Agents}} {
		if d.kind == "other" {
			return nil, fmt.Errorf("%s exists and is not a directory; move it away first", d.what)
		}
	}
	var plan []action
	add := func(say string, do func() error) { plan = append(plan, action{say, do}) }
	installed := st.unit != "" && st.record && st.tokenValid
	switch verb {
	case "install":
		if !st.user {
			return nil, fmt.Errorf("%s is not an active bus User; a runner is a person's", u.person)
		}
		changed := false
		if st.unit != want {
			changed = st.unit != ""
			add("write "+userTemplatePath, func() error { return ops.writeTemplate(want) })
		}
		if st.seen.State != "dir" || st.seen.Agents != "dir" {
			add("make "+u.state()+" and "+u.agents()+" as "+u.person+", 0700", func() error {
				_, err := ops.helper(u, "mkdirs", nil)
				return err
			})
		}
		if !st.record {
			add("create "+u.name+", "+u.person+"'s agent, allowing @owner", func() error { return ops.createRecord(u) })
		}
		if !st.tokenValid {
			changed = true
			add("write "+u.state()+"/runner.env with "+u.name+"'s token, as "+u.person+", 0600", func() error {
				t, err := ops.token(u)
				if err != nil {
					return err
				}
				_, err = ops.helper(u, "write-env", []byte(t))
				return err
			})
		}
		if !st.linger {
			add("enable linger for "+u.person+" and wait for their user manager", func() error { return ops.enableLinger(u) })
		}
		switch {
		case !st.enabled || !st.active:
			add("reload "+u.person+"'s user manager, enable and start "+userUnit, func() error {
				if err := ops.userctl(u, "daemon-reload"); err != nil {
					return err
				}
				return ops.userctl(u, "enable", "--now", userUnit)
			})
		case changed:
			add("reload "+u.person+"'s user manager and restart "+userUnit+" if running", func() error {
				if err := ops.userctl(u, "daemon-reload"); err != nil {
					return err
				}
				return ops.userctl(u, "try-restart", userUnit)
			})
		}
	case "uninstall":
		if st.enabled || st.active {
			add("stop and disable "+userUnit+" in "+u.person+"'s user manager; their tmux sessions keep running", func() error {
				return ops.userctl(u, "disable", "--now", userUnit)
			})
		}
		if st.record {
			add("remove "+u.name+" from the bus; agents it served stay registered, now unhosted", func() error { return ops.removeRecord(u) })
		}
		if st.seen.State == "dir" {
			add("remove "+u.state()+" as "+u.person, func() error {
				_, err := ops.helper(u, "remove", nil, "state")
				return err
			})
		}
		if purge && st.seen.Agents == "dir" {
			add("remove "+u.agents()+" as "+u.person, func() error {
				_, err := ops.helper(u, "remove", nil, "agents")
				return err
			})
		}
		if st.others == 0 && st.unit != "" {
			add("remove "+userTemplatePath+", the last user-account runner's", func() error { return ops.removeTemplate() })
		}
		if st.linger {
			add("leave linger for "+u.person+" as it is: it may serve other things", func() error { return nil })
		}
	case "enable":
		if !installed {
			return nil, fmt.Errorf("%s's user-account runner is not installed; --runner install --user-runner --for %s first", u.person, u.person)
		}
		if !st.enabled || !st.active {
			add("enable and start "+userUnit+" in "+u.person+"'s user manager", func() error { return ops.userctl(u, "enable", "--now", userUnit) })
		}
	case "disable":
		if st.enabled || st.active {
			add("stop and disable "+userUnit+" in "+u.person+"'s user manager, keeping its files", func() error {
				return ops.userctl(u, "disable", "--now", userUnit)
			})
		}
	default:
		return nil, fmt.Errorf("--runner %q: the verbs are install, uninstall, enable, disable and refresh", verb)
	}
	return plan, nil
}

// userRunnerVerb runs one verb for one person, or refresh for every person.
func userRunnerVerb(verb, person string, purge, dry bool) error {
	if verb != "refresh" && !runnerVerbs[verb] {
		return fmt.Errorf("--runner %q: the verbs are install, uninstall, enable, disable and refresh", verb)
	}
	if purge && verb != "uninstall" {
		return fmt.Errorf("--purge goes with --runner uninstall")
	}
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	if !dry && os.Geteuid() != 0 {
		return fmt.Errorf("a user-account runner is root's to install; run sudo %s", strings.Join(os.Args, " "))
	}
	if err := userManagerAvailable(); err != nil {
		return err
	}
	// The helper runs as the person, so it is the installed setup every
	// account can reach, never a copy unpacked somewhere private.
	self := filepath.Join(installBin, "agent-bus-setup")
	if err := reachableByAll(self); err != nil {
		return fmt.Errorf("the person must be able to run %s: %w; install the node first", self, err)
	}
	if _, err := exec.LookPath("setpriv"); err != nil {
		return fmt.Errorf("setpriv (util-linux) is needed to act as the person: %w", err)
	}
	// The person runs it, so it must be the installed program every account can
	// reach (/usr/local/bin), not a copy beside a setup unpacked under /root.
	exe := filepath.Join(installBin, "agent-bus-runner")
	if _, err := os.Stat(exe); err != nil && verb != "uninstall" {
		return fmt.Errorf("no installed %s; install the node first: %w", exe, err)
	}
	ops := hostUserOps{bus: newOwnerClient(), self: self}
	if verb == "refresh" {
		return userRefresh(host, userTemplate(exe), ops, dry)
	}
	u, err := userRunnerFor(person, host)
	if err != nil {
		return err
	}
	unlock, err := lockPerson(u.person, dry)
	if err != nil {
		return err
	}
	defer unlock()
	st, err := observeUser(u, ops, dry)
	if err != nil {
		return err
	}
	plan, err := userPlan(verb, u, st, userTemplate(exe), purge, ops)
	if err != nil {
		return err
	}
	if dry {
		fmt.Printf("(a dry run does not look inside %s; its directories and runner.env are checked when applied)\n", u.home)
	}
	return apply(u.person+"'s user-account runner", plan, dry)
}

// lockPerson keeps two setup runs for one person from applying at once.
func lockPerson(person string, dry bool) (func(), error) {
	if dry {
		return func() {}, nil
	}
	f, err := os.OpenFile("/run/agent-bus-setup-user-"+person+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another agent-bus-setup is changing %s's user-account runner; try again when it ends", person)
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

func apply(who string, plan []action, dry bool) error {
	if len(plan) == 0 {
		fmt.Printf("%s: nothing to change\n", who)
		return nil
	}
	for _, a := range plan {
		if dry {
			fmt.Println("would " + a.say)
			continue
		}
		fmt.Println(a.say)
		if err := a.do(); err != nil {
			return fmt.Errorf("%s: %w; run the same command again to resume", a.say, err)
		}
	}
	return nil
}

// reachableByAll is true when every account may traverse to and execute path.
func reachableByAll(path string) error {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	fi, err := os.Stat(real)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() || fi.Mode().Perm()&0o001 == 0 {
		return fmt.Errorf("%s must be executable by every account", real)
	}
	for dir := filepath.Dir(real); ; dir = filepath.Dir(dir) {
		di, err := os.Stat(dir)
		if err != nil {
			return err
		}
		if di.Mode().Perm()&0o001 == 0 {
			return fmt.Errorf("%s is not searchable by every account (not searchable)", dir)
		}
		if dir == "/" {
			return nil
		}
	}
}

// userManagerAvailable checks for `systemctl --user -M`, systemd 248 or later.
var userManagerAvailable = func() error {
	out, err := exec.Command("systemctl", "--version").Output()
	if err != nil {
		return fmt.Errorf("systemctl --version: %w", err)
	}
	m := regexp.MustCompile(`^systemd (\d+)`).FindSubmatch(out)
	if m == nil {
		return fmt.Errorf("systemctl --version: unexpected answer")
	}
	if n, _ := strconv.Atoi(string(m[1])); n < 248 {
		return fmt.Errorf("systemd %d is too old for `systemctl --user -M` (248 or later)", n)
	}
	return nil
}

// userRecords are the registry's user-account runners on this host, by person:
// the refresh walks these, never the home directories.
func userRecords(bus *ownerClient, host string) ([]string, error) {
	code, out, err := bus.call("GET", "/ls?kind=agent", nil)
	if err != nil || code != http.StatusOK {
		return nil, fmt.Errorf("list agents: %d %v", code, err)
	}
	var rows []struct{ Name, Owner string }
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	re := regexp.MustCompile(`^#runner/([a-z][a-z0-9_-]{0,21})-user@` + regexp.QuoteMeta(host) + `$`)
	var people []string
	for _, r := range rows {
		if m := re.FindStringSubmatch(r.Name); m != nil && r.Owner == m[1] {
			people = append(people, m[1])
		}
	}
	return people, nil
}

// userRefresh rewrites the template when it differs, then restarts each
// person's runner one by one, each with its own result.
func userRefresh(host, want string, ops hostUserOps, dry bool) error {
	people, err := userRecords(ops.bus, host)
	if err != nil {
		return err
	}
	have, _ := os.ReadFile(userTemplatePath)
	if len(people) == 0 {
		fmt.Println("user-account runners: none installed")
		return nil
	}
	if string(have) != want {
		if dry {
			fmt.Println("would write " + userTemplatePath)
		} else if err := ops.writeTemplate(want); err != nil {
			return err
		}
	}
	failed := 0
	for _, p := range people {
		u, err := userRunnerFor(p, host)
		if err == nil && !dry {
			if err = ops.userctl(u, "daemon-reload"); err == nil {
				err = ops.userctl(u, "try-restart", userUnit)
			}
		}
		switch {
		case err != nil:
			failed++
			fmt.Printf("%s's user-account runner: %v\n", p, err)
		case dry:
			fmt.Printf("would restart %s's user-account runner if running\n", p)
		default:
			fmt.Printf("%s's user-account runner: refreshed\n", p)
		}
	}
	if failed != 0 {
		return fmt.Errorf("%d of %d user-account runners not refreshed", failed, len(people))
	}
	return nil
}

// refreshUserRunners is the upgrade's last step: a failure is reported, never
// a failed upgrade, since the node itself is already upgraded.
func refreshUserRunners() {
	if _, err := os.Stat(userTemplatePath); err != nil {
		return
	}
	if err := userRunnerVerb("refresh", "", false, false); err != nil {
		fmt.Fprintln(os.Stderr, "warning: user-account runners:", err)
	}
}

// observeUser looks; it changes nothing. A dry run asks no helper.
func observeUser(u userRunner, ops hostUserOps, dry bool) (userState, error) {
	var st userState
	person, ok, err := ops.bus.lookup(u.person)
	if err != nil {
		return st, err
	}
	st.user = ok && person["kind"] == "user"
	if b, err := os.ReadFile(userTemplatePath); err == nil {
		st.unit = string(b)
	}
	record, exists, err := ops.bus.lookup(u.name)
	if err != nil {
		return st, err
	}
	st.record = exists
	if exists {
		st.recordKind, _ = record["kind"].(string)
		st.recordOwner, _ = record["owner"].(string)
	}
	if dry {
		st.seen = userSeen{State: "unknown", Agents: "unknown"}
	} else {
		out, err := ops.helper(u, "observe", nil)
		if err != nil {
			return st, err
		}
		if err := json.Unmarshal(out, &st.seen); err != nil {
			return st, fmt.Errorf("observe as %s: %w", u.person, err)
		}
	}
	if st.seen.Token != "" {
		code, out, err := ops.bus.callAs(st.seen.Token, "GET", "/status", nil)
		var s struct{ You string }
		st.tokenValid = err == nil && code == 200 && json.Unmarshal(out, &s) == nil && s.You == u.name
	}
	st.linger = lingerReady(u)
	if st.linger {
		st.enabled = ops.userctl(u, "is-enabled", "--quiet", userUnit) == nil
		st.active = ops.userctl(u, "is-active", "--quiet", userUnit) == nil
	}
	if people, err := userRecords(ops.bus, hostOf(u.name)); err == nil {
		for _, p := range people {
			if p != u.person {
				st.others++
			}
		}
	}
	return st, nil
}

// lingerReady: the linger flag alone is not enough; the person's user manager
// must be running (its bus present), or install runs enable-linger again,
// which starts it. Setup never starts or stops user@<uid>.service itself.
var lingerDir, runUser = "/var/lib/systemd/linger", "/run/user"

func lingerReady(u userRunner) bool {
	_, flag := os.Stat(filepath.Join(lingerDir, u.person))
	_, bus := os.Stat(filepath.Join(runUser, strconv.Itoa(u.uid), "bus"))
	return flag == nil && bus == nil
}

func hostOf(name string) string { return name[strings.LastIndex(name, "@")+1:] }

// hostUserOps does it on this host.
type hostUserOps struct {
	bus  *ownerClient
	self string
}

func (o hostUserOps) helper(u userRunner, op string, stdin []byte, args ...string) ([]byte, error) {
	argv := helperArgv(o.self, u, op, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = helperEnv(u)
	cmd.Dir = "/"
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s as %s: %v: %s", op, u.person, err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

func (o hostUserOps) writeTemplate(unit string) error {
	if err := os.MkdirAll(filepath.Dir(userTemplatePath), 0o755); err != nil {
		return err
	}
	tmp := userTemplatePath + ".new"
	if err := os.WriteFile(tmp, []byte(unit), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, userTemplatePath)
}

func (o hostUserOps) removeTemplate() error {
	if err := os.Remove(userTemplatePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// enableLinger starts the person's user manager without a login and waits,
// bounded, for its bus.
func (o hostUserOps) enableLinger(u userRunner) error {
	if err := run("loginctl", "enable-linger", u.person); err != nil {
		return err
	}
	bus := filepath.Join(runUser, strconv.Itoa(u.uid), "bus")
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		if _, err := os.Stat(bus); err == nil {
			return nil
		}
	}
	return fmt.Errorf("%s's user manager did not start: no %s", u.person, bus)
}

func (o hostUserOps) userctl(u userRunner, args ...string) error {
	argv, err := userCtlArgv(u.person, args...)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if len(args) > 1 && args[1] == "--quiet" {
		return cmd.Run()
	}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func (o hostUserOps) createRecord(u userRunner) error {
	return o.bus.mustAs(u.person, "create "+u.name, "POST", "/register", map[string]any{
		"name": u.name, "kind": "agent", "allow": []string{"@owner"},
		"descr": u.person + "'s user-account runner: status, reload, start, stop, restart, enable, disable, read, write, create, delete",
	})
}

func (o hostUserOps) removeRecord(u userRunner) error {
	return o.bus.must("remove "+u.name, "POST", "/unregister", map[string]any{"name": u.name}, 404)
}

// token is the runner agent's own credential, never the person's.
func (o hostUserOps) token(u userRunner) (string, error) {
	code, out, err := o.bus.call("POST", "/token", map[string]any{"name": u.name})
	var t struct{ Token string }
	if err != nil || code != 200 || json.Unmarshal(out, &t) != nil || t.Token == "" {
		return "", fmt.Errorf("no token for %s: %d", u.name, code)
	}
	return t.Token, nil
}

// userHelper is this program re-executed as the person by setpriv: it works
// only under their home, creates nothing readable by others and never
// prints the token anywhere but its stdout pipe to root.
func userHelper(args []string) error {
	syscall.Umask(0o077)
	if len(args) == 0 {
		return fmt.Errorf("helper: no operation")
	}
	home := os.Getenv("HOME")
	if !filepath.IsAbs(home) || os.Geteuid() == 0 {
		return fmt.Errorf("helper: runs as the person, with their HOME")
	}
	state, agents := filepath.Join(home, userStateRel), filepath.Join(home, userAgentsRel)
	switch args[0] {
	case "observe":
		seen := userSeen{State: kindOf(state), Agents: kindOf(agents)}
		if seen.State == "dir" {
			seen.Token = envToken(filepath.Join(state, "runner.env"))
		}
		return json.NewEncoder(os.Stdout).Encode(seen)
	case "mkdirs":
		for _, d := range []string{state, agents} {
			if err := os.MkdirAll(d, 0o700); err != nil {
				return err
			}
			if kindOf(d) != "dir" {
				return fmt.Errorf("%s is not a directory", d)
			}
			if err := os.Chmod(d, 0o700); err != nil {
				return err
			}
		}
		return nil
	case "write-env":
		token, err := io.ReadAll(io.LimitReader(os.Stdin, 4096))
		if err != nil {
			return err
		}
		t := strings.TrimSpace(string(token))
		if _, err := hex.DecodeString(t); t == "" || err != nil {
			return fmt.Errorf("helper: no usable token on stdin")
		}
		f, err := os.CreateTemp(state, ".runner-env-*")
		if err != nil {
			return err
		}
		defer os.Remove(f.Name())
		if _, err := f.WriteString("AGENT_BUS_TOKEN=" + t + "\n"); err != nil {
			f.Close()
			return err
		}
		if err := f.Sync(); err != nil { // never an empty runner.env after a power loss
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		return os.Rename(f.Name(), filepath.Join(state, "runner.env"))
	case "remove":
		target := map[string]string{"state": state, "agents": agents}[strings.Join(args[1:], "")]
		if target == "" {
			return fmt.Errorf("helper: remove state or agents")
		}
		// RemoveAll removes a link, never what it points to.
		return os.RemoveAll(target)
	}
	return fmt.Errorf("helper: unknown operation %q", args[0])
}

// kindOf names a path without following a final link.
func kindOf(path string) string {
	fi, err := os.Lstat(path)
	switch {
	case err != nil:
		return "missing"
	case fi.IsDir():
		return "dir"
	default:
		return "other"
	}
}
