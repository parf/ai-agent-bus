package main

import (
	"encoding/hex"
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

	"github.com/parf/ai-agent-bus/internal/envfile"
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
	sharedAgents       = stateRoot + "/agent.d"
	runnerTemplatePath = "/etc/systemd/system/agent-bus-runner@.service"
)

// runnerVerbs are the four, and nothing else is one.
var runnerVerbs = map[string]bool{"install": true, "uninstall": true, "enable": true, "disable": true}

// A person's name becomes part of a Linux account's, which allows 32
// characters and a narrow alphabet.
// It begins with a letter, as every distribution's useradd accepts.
var personName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,21}$`)

// personRunner is where one person's runner lives.
type personRunner struct {
	person  string // the bus User
	account string // <person>-ab-runner
	agents  string // shared public code, root-owned
	private string // stateRoot/<account>, the runner account's alone
	name    string // #runner/<person>@<host>
	unit    string // agent-bus-runner@<person>.service
}

func runnerFor(person, host string) (personRunner, error) {
	if !personName.MatchString(person) {
		return personRunner{}, fmt.Errorf("--for %q: a person's runner needs a plain name, a lowercase letter, then letters, digits, _ and -, at most 22 characters", person)
	}
	return personRunner{
		person: person, account: person + "-ab-runner",
		agents: sharedAgents, private: filepath.Join(stateRoot, person+"-ab-runner"),
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
ConditionPathExists=%[2]s/%%i-ab-runner/runner.env

[Service]
User=%%i-ab-runner
EnvironmentFile=%[2]s/%%i-ab-runner/runner.env
ExecStart=%[1]s --agents %[2]s/agent.d --host %[2]s/%%i-ab-runner
WorkingDirectory=%[2]s/%%i-ab-runner
Restart=on-failure
RestartSec=2
# Give the runner its normal ten-second keeper drain before the host ceiling.
KillMode=mixed
TimeoutStopSec=30s
# The runner gives each agent a cgroup of its own inside this subtree
# (Plans/R1.0-Release/runner.md#sandboxing); no capability is needed for it.
Delegate=yes
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=%[2]s/%%i-ab-runner

[Install]
WantedBy=multi-user.target
`, exe, stateRoot)
}

// dirState is one directory as found.
type dirState struct {
	exists bool
	owner  string
	mode   os.FileMode
}

// runnerState is the host as one verb finds it.
type runnerState struct {
	user        bool // the person is an active bus User
	account     bool
	root        dirState
	agents      dirState
	private     dirState
	unit        string // the template on disk, "" when absent
	record      bool
	recordKind  string
	recordOwner string
	envValid    bool // runner.env holds a token that is the runner's agent
	enabled     bool
	active      bool
	others      int // other persons' runners on this host
}

// action is one planned step: said always, done unless it is a dry run.
type action struct {
	say string
	do  func() error
}

// runnerPlan is what verb needs on a host in state st; empty when nothing does.
func runnerPlan(verb string, p personRunner, st runnerState, want string, purge bool, ops runnerOps) ([]action, error) {
	if (verb == "install" || verb == "enable") && st.record &&
		(st.recordKind != "agent" || st.recordOwner != p.person) {
		return nil, fmt.Errorf("%s is not %s's agent; refusing to use it as their runner", p.name, p.person)
	}
	var plan []action
	add := func(say string, do func() error) { plan = append(plan, action{say, do}) }
	installed := st.account && st.unit != "" && st.record && st.envValid
	switch verb {
	case "install":
		if !st.user {
			return nil, fmt.Errorf("%s is not an active bus User; a runner is a person's", p.person)
		}
		if !st.account {
			add("create the system account "+p.account+" with home "+p.private, func() error { return ops.addAccount(p) })
		}
		for _, d := range []struct {
			path, owner string
			mode        os.FileMode
			is          dirState
		}{
			{stateRoot, "root", 0o755, st.root},
			{p.agents, "root", 0o755, st.agents},
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
		// Shared code is never a person's uninstall target, even with --purge.
		if st.private.exists {
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

// runnerLayoutRoot is fixed in production; tests use a disposable legacy tree.
var runnerLayoutRoot = stateRoot

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
	if err := refuseLegacyRunners(runnerLayoutRoot); err != nil {
		return err
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
		return fmt.Errorf("a person's runner is root's to install; run %s", sudoLine())
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
	st, err := observeRunner(p, ops)
	if err != nil {
		return err
	}
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
		"descr": p.person + "'s runner: status, reload, start, stop, restart, enable, disable, read, write, create, delete",
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
	account, err := user.Lookup(p.account)
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(account.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(account.Gid)
	if err != nil {
		return err
	}
	// The directory belongs to the runner: never follow or chown a pathname
	// it can plant. CreateTemp uses O_EXCL; ownership is applied to its fd.
	f, err := os.CreateTemp(p.private, ".runner-env-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := f.Chown(uid, gid); err != nil {
		return err
	}
	if _, err := f.WriteString("AGENT_BUS_TOKEN=" + t.Token + "\n"); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), p.envFile())
}

// observeRunner looks; it changes nothing.
func observeRunner(p personRunner, o runnerOps) (runnerState, error) {
	var st runnerState
	// A lookup answers only for what is active, so an inactive User is no
	// User here either; a record of another kind under the name is not one.
	person, ok, err := o.bus.lookup(p.person)
	if err != nil {
		return st, err
	}
	st.user = ok && person["kind"] == "user"
	_, err = user.Lookup(p.account)
	st.account = err == nil
	st.root, st.agents, st.private = dirAt(stateRoot), dirAt(p.agents), dirAt(p.private)
	if b, err := os.ReadFile(runnerTemplatePath); err == nil {
		st.unit = string(b)
	}
	record, exists, err := o.bus.lookup(p.name)
	if err != nil {
		return st, err
	}
	st.record = exists
	if exists {
		st.recordKind, _ = record["kind"].(string)
		st.recordOwner, _ = record["owner"].(string)
	}
	if tok := envToken(p.envFile()); tok != "" {
		code, out, err := o.bus.callAs(tok, "GET", "/status", nil)
		var s struct{ You string }
		st.envValid = err == nil && code == 200 && json.Unmarshal(out, &s) == nil && s.You == p.name
	}
	st.enabled = exec.Command("systemctl", "is-enabled", "--quiet", p.unit).Run() == nil
	st.active = exec.Command("systemctl", "is-active", "--quiet", p.unit).Run() == nil
	others, _ := filepath.Glob(filepath.Join(stateRoot, "*-ab-runner", "runner.env"))
	for _, f := range others {
		if f != p.envFile() {
			st.others++
		}
	}
	return st, nil
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

// envToken reads setup's single-token file with the shared ENV parser. Keep
// this machine-written subset identical for the parser and systemd.
func envToken(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	vars, err := envfile.Parse(b)
	if err != nil || len(vars) != 1 || vars[0].Key != "AGENT_BUS_TOKEN" {
		return ""
	}
	token := vars[0].Value
	if token == "" {
		return ""
	}
	if _, err := hex.DecodeString(token); err != nil {
		return ""
	}
	// Noncanonical files are rewritten by install, not trusted by one reader
	// while interpreted differently by systemd's EnvironmentFile.
	if string(b) != "AGENT_BUS_TOKEN="+token+"\n" && string(b) != "AGENT_BUS_TOKEN="+token {
		return ""
	}
	return token
}

// The only old-layout installation was the unreleased development node.
// Refuse before observing the bus or planning host changes; migration is manual.
func refuseLegacyRunners(root string) error {
	path := filepath.Join(root, "runners")
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("legacy runner tree %s: move it to the new layout (docs/07-setup.md#a-persons-runner) and run again", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
