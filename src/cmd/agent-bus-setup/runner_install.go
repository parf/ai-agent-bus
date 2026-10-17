package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

// A person's runner (runner wave M.11,
// Plans/R1.0-Release/TODO.md#runner-waves): `--runner install|uninstall|
// enable|disable --for <person>`. Each person gets a Linux account of their
// own, <person>-ab-runner, so their secrets stay apart from everybody else's;
// an instance of one template unit; and their own agent on the bus,
// #runner/<person>@<host>, whose token the runner is started with
// (docs/07-setup.md#a-persons-runner). Nothing of it is installed by default.
//
// Every verb looks at the host, plans what is missing, and applies only that,
// so a second run plans nothing and --dry-run prints the plan.

const (
	runnersRoot        = stateRoot + "/runners"
	runnerTemplatePath = "/etc/systemd/system/agent-bus-runner@.service"
)

// runnerVerbs are the four, and nothing else is one.
var runnerVerbs = map[string]bool{"install": true, "uninstall": true, "enable": true, "disable": true}

// A person's name becomes part of a Linux account's, which allows 32
// characters and a narrow alphabet.
var personName = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,21}$`)

// personRunner is where one person's runner lives.
type personRunner struct {
	person  string // the bus User
	account string // <person>-ab-runner
	home    string // runners/<person>
	agents  string // home/agent.d, the person's to write
	private string // home/runner, the runner account's alone
	name    string // #runner/<person>@<host>
	unit    string // agent-bus-runner@<person>.service
}

func runnerFor(person, host string) (personRunner, error) {
	if !personName.MatchString(person) {
		return personRunner{}, fmt.Errorf("--for %q: a person's runner needs a plain name, lowercase letters, digits, _ and -, at most 22 characters", person)
	}
	home := filepath.Join(runnersRoot, person)
	return personRunner{
		person: person, account: person + "-ab-runner", home: home,
		agents: filepath.Join(home, "agent.d"), private: filepath.Join(home, "runner"),
		name: "#runner/" + person + "@" + host, unit: "agent-bus-runner@" + person + ".service",
	}, nil
}

func (p personRunner) envFile() string { return filepath.Join(p.private, "runner.env") }

// runnerTemplate is the one unit every person's runner is an instance of:
// the person's own account, only its private directory writable, a cgroup
// subtree delegated to it, no capability, and no start while it has no token.
// Wanted beside a local bus and ordered after it, never Required.
func runnerTemplate(exe string) string {
	return fmt.Sprintf(`[Unit]
Description=agent-bus runner for %%i
Documentation=https://github.com/parf/ai-agent-bus/blob/main/docs/07-setup.md#a-persons-runner
Wants=agent-busd.service
After=agent-busd.service
ConditionPathExists=%[2]s/%%i/runner/runner.env

[Service]
User=%%i-ab-runner
EnvironmentFile=%[2]s/%%i/runner/runner.env
ExecStart=%[1]s --home %[2]s/%%i
WorkingDirectory=%[2]s/%%i/runner
Restart=on-failure
RestartSec=2
# The runner gives each agent a cgroup of its own inside this subtree
# (Plans/R1.0-Release/runner.md#sandboxing); no capability is needed for it.
Delegate=yes
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=%[2]s/%%i/runner

[Install]
WantedBy=multi-user.target
`, exe, runnersRoot)
}

// dirState is one directory as found.
type dirState struct {
	exists bool
	owner  string
	mode   os.FileMode
}

// runnerState is the host as one verb finds it.
type runnerState struct {
	user          bool // the person is an active bus User
	account       bool
	root, home    dirState
	agents        dirState
	private       dirState
	unit          string // the template on disk, "" when absent
	record        bool
	envValid      bool // runner.env holds a token that is the runner's agent
	enabled       bool
	active        bool
	others        int  // other persons' runners on this host
	personAccount bool // a Linux account of the person's own name, to own agent.d
}

// action is one planned step: said always, done unless it is a dry run.
type action struct {
	say string
	do  func() error
}

// runnerPlan is what verb needs on a host in state st; empty when nothing does.
func runnerPlan(verb string, p personRunner, st runnerState, want string, purge bool, ops runnerOps) ([]action, error) {
	var plan []action
	add := func(say string, do func() error) { plan = append(plan, action{say, do}) }
	installed := st.account && st.unit != "" && st.envValid
	switch verb {
	case "install":
		if !st.user {
			return nil, fmt.Errorf("%s is not an active bus User; a runner is a person's", p.person)
		}
		if !st.account {
			add("create the system account "+p.account+" with home "+p.private, func() error { return ops.addAccount(p) })
		}
		agentsOwner := "root"
		if st.personAccount {
			agentsOwner = p.person
		}
		for _, d := range []struct {
			path, owner string
			mode        os.FileMode
			is          dirState
		}{
			{runnersRoot, "root", 0o755, st.root},
			{p.home, "root", 0o755, st.home},
			{p.agents, agentsOwner, 0o755, st.agents},
			{p.private, p.account, 0o700, st.private},
		} {
			if !d.is.exists || d.is.owner != d.owner || d.is.mode != d.mode {
				d := d
				add(fmt.Sprintf("make %s %s's own, %#o", d.path, d.owner, d.mode), func() error { return ops.makeDir(d.path, d.owner, d.mode) })
			}
		}
		changed := false
		if st.unit != want {
			changed = st.unit != ""
			add("write "+runnerTemplatePath+" and reload systemd", func() error { return ops.writeTemplate(want) })
		}
		if !st.record {
			add("create "+p.name+", "+p.person+"'s agent, allowing @owner", func() error { return ops.createRecord(p) })
		}
		if !st.envValid {
			changed = true
			add("write "+p.envFile()+" with "+p.name+"'s token, "+p.account+"'s alone", func() error { return ops.writeEnv(p) })
		}
		switch {
		case !st.enabled || !st.active:
			add("enable and start "+p.unit, func() error { return ops.systemctl("enable", "--now", p.unit) })
		case changed:
			add("restart "+p.unit, func() error { return ops.systemctl("restart", p.unit) })
		}
	case "uninstall":
		if st.enabled || st.active {
			add("stop and disable "+p.unit, func() error { return ops.systemctl("disable", "--now", p.unit) })
		}
		if st.record {
			add("remove "+p.name+" from the bus; the agents it created stay "+p.person+"'s", func() error { return ops.removeRecord(p) })
		}
		if purge && st.home.exists {
			add("remove "+p.home+", agent.d included", func() error { return os.RemoveAll(p.home) })
		} else if st.private.exists {
			add("remove "+p.private+" and keep "+p.agents, func() error { return os.RemoveAll(p.private) })
		}
		if st.account {
			add("delete the system account "+p.account, func() error { return ops.run("userdel", p.account) })
		}
		if st.others == 0 && st.unit != "" {
			add("remove "+runnerTemplatePath+", the last runner's", func() error { return ops.removeTemplate() })
		}
	case "enable":
		if !installed {
			return nil, fmt.Errorf("%s's runner is not installed; --runner install --for %s first", p.person, p.person)
		}
		if !st.enabled || !st.active {
			add("enable and start "+p.unit, func() error { return ops.systemctl("enable", "--now", p.unit) })
		}
	case "disable":
		if st.enabled || st.active {
			add("stop and disable "+p.unit+", keeping its files", func() error { return ops.systemctl("disable", "--now", p.unit) })
		}
	default:
		return nil, fmt.Errorf("--runner %q: the verbs are install, uninstall, enable and disable", verb)
	}
	return plan, nil
}

// runnerVerb runs one verb for one person.
func runnerVerb(verb, person string, purge, dry bool) error {
	if !runnerVerbs[verb] {
		return fmt.Errorf("--runner %q: the verbs are install, uninstall, enable and disable", verb)
	}
	if purge && verb != "uninstall" {
		return fmt.Errorf("--purge goes with --runner uninstall")
	}
	if person == "" {
		return fmt.Errorf("--for: whose runner")
	}
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	p, err := runnerFor(person, host)
	if err != nil {
		return err
	}
	if !dry && os.Geteuid() != 0 {
		return fmt.Errorf("a person's runner is root's to install; run sudo %s", strings.Join(os.Args, " "))
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	exe := filepath.Join(filepath.Dir(self), "agent-bus-runner")
	if _, err := os.Stat(exe); err != nil && verb == "install" {
		return fmt.Errorf("no runner beside agent-bus-setup: %w", err)
	}
	ops := runnerOps{bus: newOwnerClient()}
	st := observeRunner(p, ops)
	plan, err := runnerPlan(verb, p, st, runnerTemplate(exe), purge, ops)
	if err != nil {
		return err
	}
	if len(plan) == 0 {
		fmt.Printf("%s's runner: nothing to change\n", p.person)
		return nil
	}
	for _, a := range plan {
		if dry {
			fmt.Println("would " + a.say)
			continue
		}
		fmt.Println(a.say)
		if err := a.do(); err != nil {
			return fmt.Errorf("%s: %w", a.say, err)
		}
	}
	return nil
}

// runnerOps is what the verbs do to the host and the bus.
type runnerOps struct{ bus *ownerClient }

func (o runnerOps) run(name string, args ...string) error { return run(name, args...) }

func (o runnerOps) systemctl(args ...string) error { return run("systemctl", args...) }

func (o runnerOps) addAccount(p personRunner) error {
	return run("useradd", "--system", "--home-dir", p.private, "--no-create-home", "--shell", "/usr/sbin/nologin", p.account)
}

func (o runnerOps) makeDir(path, owner string, mode os.FileMode) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	if err := run("chown", owner+":"+owner, path); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

func (o runnerOps) writeTemplate(unit string) error {
	if err := os.WriteFile(runnerTemplatePath, []byte(unit), 0o644); err != nil {
		return err
	}
	return run("systemctl", "daemon-reload")
}

func (o runnerOps) removeTemplate() error {
	if err := os.Remove(runnerTemplatePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return run("systemctl", "daemon-reload")
}

// createRecord registers the runner's agent as its person, so it is theirs
// from the start and its ACL is theirs to change.
func (o runnerOps) createRecord(p personRunner) error {
	return o.bus.mustAs(p.person, "create "+p.name, "POST", "/register", map[string]any{
		"name": p.name, "kind": "agent", "allow": []string{"@owner"},
		"descr": p.person + "'s runner: status, reload",
	})
}

func (o runnerOps) removeRecord(p personRunner) error {
	return o.bus.must("remove "+p.name, "POST", "/unregister", map[string]any{"name": p.name}, 404)
}

// writeEnv puts the runner's agent's token where only its account reads it,
// whole or not at all.
func (o runnerOps) writeEnv(p personRunner) error {
	code, out, err := o.bus.call("POST", "/token", map[string]any{"name": p.name})
	var t struct{ Token string }
	if err != nil || code != 200 || json.Unmarshal(out, &t) != nil || t.Token == "" {
		return fmt.Errorf("no token for %s: %d %s", p.name, code, strings.TrimSpace(string(out)))
	}
	tmp := p.envFile() + ".new"
	if err := os.WriteFile(tmp, []byte("AGENT_BUS_TOKEN="+t.Token+"\n"), 0o600); err != nil {
		return err
	}
	if err := run("chown", p.account+":"+p.account, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, p.envFile())
}

// observeRunner looks; it changes nothing.
func observeRunner(p personRunner, o runnerOps) runnerState {
	var st runnerState
	// A lookup answers only for what is active, so an inactive User is no
	// User here either; a record of another kind under the name is not one.
	if rec, ok := o.bus.lookup(p.person); ok && rec["kind"] == "user" {
		st.user = true
	}
	_, err := user.Lookup(p.account)
	st.account = err == nil
	_, err = user.Lookup(p.person)
	st.personAccount = err == nil
	st.root, st.home, st.agents, st.private = dirAt(runnersRoot), dirAt(p.home), dirAt(p.agents), dirAt(p.private)
	if b, err := os.ReadFile(runnerTemplatePath); err == nil {
		st.unit = string(b)
	}
	_, st.record = o.bus.lookup(p.name)
	if tok := envToken(p.envFile()); tok != "" {
		code, out, err := o.bus.callAs(tok, "GET", "/status", nil)
		var s struct{ You string }
		st.envValid = err == nil && code == 200 && json.Unmarshal(out, &s) == nil && s.You == p.name
	}
	st.enabled = exec.Command("systemctl", "is-enabled", "--quiet", p.unit).Run() == nil
	st.active = exec.Command("systemctl", "is-active", "--quiet", p.unit).Run() == nil
	others, _ := filepath.Glob(filepath.Join(runnersRoot, "*", "runner", "runner.env"))
	for _, f := range others {
		if f != p.envFile() {
			st.others++
		}
	}
	return st
}

func dirAt(path string) dirState {
	fi, err := os.Stat(path)
	if err != nil || !fi.IsDir() {
		return dirState{}
	}
	d := dirState{exists: true, mode: fi.Mode().Perm()}
	if sys, ok := fi.Sys().(*syscall.Stat_t); ok {
		if u, err := user.LookupId(strconv.Itoa(int(sys.Uid))); err == nil {
			d.owner = u.Username
		}
	}
	return d
}

// envToken is the AGENT_BUS_TOKEN a runner.env holds, or "".
func envToken(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "AGENT_BUS_TOKEN="); ok {
			return v
		}
	}
	return ""
}
