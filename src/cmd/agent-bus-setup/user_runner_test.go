package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestUserControlReachesTheTargetBusWithNoRootEnvironment(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.jsonl")
	stub := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/usr/bin/python3\n"+text), 0700); err != nil {
			t.Fatal(err)
		}
	}
	stub("setpriv", fmt.Sprintf(`import json, os, sys
assert sys.argv[1:6] == ['--reuid=1001', '--regid=1002', '--init-groups', '--no-new-privs', '--'], sys.argv
assert os.getcwd() == '/', os.getcwd()
with open(%q, 'a') as f: f.write(json.dumps({'identity': sys.argv[1:6], 'keys': sorted(os.environ), 'env': {k: os.environ.get(k) for k in ['HOME', 'PATH', 'LANG', 'XDG_RUNTIME_DIR', 'DBUS_SESSION_BUS_ADDRESS']}})+'\n')
os.execv(sys.argv[6], sys.argv[6:])
`, log))
	stub("systemctl", fmt.Sprintf(`import json, os, sys
assert '-M' not in sys.argv, 'transient-machine transport refused'
assert os.environ.get('XDG_RUNTIME_DIR') == '/run/user/1001', 'wrong target runtime directory'
assert os.environ.get('DBUS_SESSION_BUS_ADDRESS') == 'unix:path=/run/user/1001/bus', 'wrong target bus'
with open(%q, 'a') as f: f.write(json.dumps({'args': sys.argv[1:]})+'\n')
`, log))
	was := toolDirs
	toolDirs = []string{dir}
	t.Cleanup(func() { toolDirs = was })
	for key, value := range map[string]string{"HOME": "/root-fixture", "XDG_RUNTIME_DIR": "/run/user/0",
		"DBUS_SESSION_BUS_ADDRESS": "unix:path=/root/wrong-bus", "AGENT_BUS_TOKEN": "root-fixture-secret", "SUDO_USER": "root"} {
		t.Setenv(key, value)
	}
	u := userRunner{person: "alice", uid: 1001, gid: 1002, home: "/home/alice"}
	for _, args := range [][]string{{"daemon-reload"}, {"try-restart", userUnit}, {"is-active", "--quiet", userUnit}} {
		if err := (hostUserOps{}).userctl(u, args...); err != nil {
			t.Fatalf("target-user bus command %q failed: %v", args, err)
		}
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 6 {
		t.Fatalf("want three identity hops and three commands, got %q", data)
	}
	wantArgs := []string{"--user daemon-reload", "--user try-restart " + userUnit, "--user is-active --quiet " + userUnit}
	for i := 0; i < len(lines); i += 2 {
		var row struct {
			Env  map[string]string
			Keys []string
		}
		if err := json.Unmarshal([]byte(lines[i]), &row); err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"HOME": "/home/alice", "PATH": "/usr/local/bin:/usr/bin:/bin", "LANG": "C.UTF-8",
			"XDG_RUNTIME_DIR": "/run/user/1001", "DBUS_SESSION_BUS_ADDRESS": "unix:path=/run/user/1001/bus"}
		if len(row.Keys) != len(want) {
			t.Fatalf("inherited environment: %d variables, want %d", len(row.Keys), len(want))
		}
		for k, v := range want {
			if row.Env[k] != v {
				t.Errorf("%s=%q, want %q", k, row.Env[k], v)
			}
		}
		var command struct{ Args []string }
		if err := json.Unmarshal([]byte(lines[i+1]), &command); err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(command.Args, " "); got != wantArgs[i/2] {
			t.Fatalf("wrong user-manager command %q, want %q", got, wantArgs[i/2])
		}
	}
	if err := (hostUserOps{}).userctl(u, "stop", "ab-session-work.scope"); err == nil {
		t.Fatal("accepted a session command")
	}
	after, _ := os.ReadFile(log)
	if string(after) != string(data) {
		t.Fatal("a refused command reached systemctl")
	}
}

func TestGitInstallUserRefreshFailureHasManualInstruction(t *testing.T) {
	data, err := os.ReadFile("../../git-install.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "if [ -f /etc/systemd/user/agent-bus-runner.service ]; then")
	if start < 0 {
		t.Fatal("missing user refresh block")
	}
	end := strings.Index(text[start:], "\necho\n")
	if end < 0 {
		t.Fatal("missing user refresh block end")
	}
	block := strings.Replace(text[start:start+end], "[ -f /etc/systemd/user/agent-bus-runner.service ]", "true", 1)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agent-bus-setup"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", "set -euo pipefail\nbindir=$1\n"+block+"\necho deploy-continued\n", "test", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unreachable user manager failed the deploy: %v: %s", err, out)
	}
	for _, want := range []string{"deploy-continued", "person's login", "systemctl --user daemon-reload", "systemctl --user try-restart agent-bus-runner.service"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing manual recovery %q in %s", want, out)
		}
	}
}

func fakeAccounts(t *testing.T, accounts map[string]account) {
	t.Helper()
	oldP, oldU := passwd, uidMin
	passwd = func(name string) (account, bool, error) { a, ok := accounts[name]; return a, ok, nil }
	uidMin = func() int { return 1000 }
	t.Cleanup(func() { passwd, uidMin = oldP, oldU })
}

func TestUserRunnerPersonIsAnOrdinaryLoginAccount(t *testing.T) {
	fakeAccounts(t, map[string]account{
		"alice":  {uid: 1001, gid: 1001, home: "/home/alice", shell: "/bin/bash"},
		"root":   {uid: 0, gid: 0, home: "/root", shell: "/bin/bash"},
		"daemon": {uid: 2, gid: 2, home: "/", shell: "/bin/bash"},
		"svc":    {uid: 990, gid: 990, home: "/var/lib/svc", shell: "/bin/bash"},
		"locked": {uid: 1002, gid: 1002, home: "/home/locked", shell: "/usr/sbin/nologin"},
		"nope":   {uid: 1003, gid: 1003, home: "/home/nope", shell: "/bin/false"},
	})
	u, err := userRunnerFor("alice", "node.example")
	if err != nil || u.name != "#runner/alice-user@node.example" || u.uid != 1001 || u.state() != "/home/alice/.local/state/agent-bus/runner" {
		t.Fatalf("alice: %+v %v", u, err)
	}
	for person, why := range map[string]string{"root": "refusing uid 0", "daemon": "system account", "svc": "system account",
		"locked": "not a login account", "nope": "not a login account", "ghost": "no Linux account",
		"Alice": "plain name", "a/b": "plain name", "..": "plain name", "": "SUDO_USER"} {
		if _, err := userRunnerFor(person, "node.example"); err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("--for %q: %v, want %q", person, err, why)
		}
	}
}

func TestUserManagerIsOnlyAskedAboutTheRunnerUnit(t *testing.T) {
	argv, err := userCtlArgv("enable", "--now", userUnit)
	if err != nil || strings.Join(argv, " ") != tool("systemctl")+" --user enable --now agent-bus-runner.service" {
		t.Fatalf("%q %v", argv, err)
	}
	if _, err := userCtlArgv("daemon-reload"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]string{{"restart", "user@1001.service"}, {"stop", userUnit}, {"restart", userUnit},
		{"daemon-reexec"}, {"stop", "ab-session-work.scope"}, {"kill", userUnit}, {"try-restart", "ab-session-work.scope"},
		{"enable"}, {"try-restart", userUnit, "user@1001.service"}, {"enable", "--now", "ab-session-work.scope", userUnit}} {
		if _, err := userCtlArgv(bad...); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestSetupNeverStopsAUserManagerOrItsSessions(t *testing.T) {
	forbidden := regexp.MustCompile(`terminate-user|kill-user|daemon-reexec|"user@[^"]*\.service"|ab-session-`)
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if m := forbidden.Find(b); m != nil {
			t.Errorf("%s contains %q", f, m)
		}
	}
}

func TestUserTemplateKeepsSessionsAndBoundsCrashes(t *testing.T) {
	unit := userTemplate("/usr/local/bin/agent-bus-runner")
	for _, want := range []string{"Delegate=yes", "KillMode=mixed", "RestartSec=", "StartLimitBurst=", "TimeoutStopSec=",
		"ConditionPathExists=%h/.local/state/agent-bus/runner/runner.env",
		"ExecStart=/usr/local/bin/agent-bus-runner --agents %h/.local/share/agent-bus/agent.d --host %h/.local/state/agent-bus/runner",
		"WantedBy=default.target"} {
		if !strings.Contains(unit, want) {
			t.Errorf("template lacks %q", want)
		}
	}
	for _, bad := range []string{"PrivateTmp", "ProtectHome", "RuntimeDirectory", "/var/lib/agent-bus/agent.d", "KillMode=control-group"} {
		if strings.Contains(unit, bad) {
			t.Errorf("template has %q", bad)
		}
	}
}

func TestHelperRunsAsThePersonWithNothingInherited(t *testing.T) {
	u := userRunner{person: "alice", uid: 1001, gid: 1002, home: "/home/alice"}
	argv := strings.Join(helperArgv("/usr/local/bin/agent-bus-setup", u, "observe"), " ")
	if argv != tool("setpriv")+" --reuid=1001 --regid=1002 --init-groups --no-new-privs -- /usr/local/bin/agent-bus-setup --user-runner-helper observe" {
		t.Fatal(argv)
	}
	if env := strings.Join(helperEnv(u), " "); env != "HOME=/home/alice PATH=/usr/local/bin:/usr/bin:/bin LANG=C.UTF-8" {
		t.Fatal(env)
	}
}

// fakeUserOps records what a plan does instead of doing it.
type fakeUserOps struct{ did []string }

func (f *fakeUserOps) helper(u userRunner, op string, stdin []byte, args ...string) ([]byte, error) {
	f.did = append(f.did, "helper "+op+" "+strings.Join(args, " "))
	return nil, nil
}
func (f *fakeUserOps) writeTemplate(string) error    { f.did = append(f.did, "template"); return nil }
func (f *fakeUserOps) removeTemplate() error         { f.did = append(f.did, "remove template"); return nil }
func (f *fakeUserOps) enableLinger(userRunner) error { f.did = append(f.did, "linger"); return nil }
func (f *fakeUserOps) userctl(u userRunner, args ...string) error {
	f.did = append(f.did, "userctl "+strings.Join(args, " "))
	return nil
}
func (f *fakeUserOps) createRecord(userRunner) error { f.did = append(f.did, "record"); return nil }
func (f *fakeUserOps) removeRecord(userRunner) error { f.did = append(f.did, "unregister"); return nil }
func (f *fakeUserOps) token(userRunner) (string, error) {
	f.did = append(f.did, "token")
	return "t0k", nil
}

func runPlan(t *testing.T, verb string, st userState, purge bool) []string {
	t.Helper()
	ops := &fakeUserOps{}
	u := userRunner{person: "alice", uid: 1001, gid: 1001, home: "/home/alice", name: "#runner/alice-user@n"}
	plan, err := userPlan(verb, u, st, userTemplate("/r"), purge, ops)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range plan {
		if err := a.do(); err != nil {
			t.Fatal(err)
		}
	}
	return ops.did
}

func TestUserPlanInstallsOnceAndUninstallsTheRest(t *testing.T) {
	fresh := userState{user: true, seen: userSeen{State: "missing", Agents: "missing"}}
	got := strings.Join(runPlan(t, "install", fresh, false), "; ")
	want := "template; helper mkdirs ; record; token; helper write-env ; linger; userctl daemon-reload; userctl enable --now agent-bus-runner.service"
	if got != want {
		t.Fatalf("install:\n got %s\nwant %s", got, want)
	}
	done := userState{user: true, seen: userSeen{State: "dir", Agents: "dir", Token: "t"}, unit: userTemplate("/r"),
		record: true, recordKind: "agent", recordOwner: "alice", tokenValid: true, linger: true, enabled: true, active: true}
	if got := runPlan(t, "install", done, false); len(got) != 0 {
		t.Fatalf("second install: %q", got)
	}
	stale := done
	stale.unit = "old"
	if got := strings.Join(runPlan(t, "install", stale, false), "; "); got != "template; userctl daemon-reload; userctl try-restart agent-bus-runner.service" {
		t.Fatalf("template refresh: %s", got)
	}
	if got := strings.Join(runPlan(t, "uninstall", done, false), "; "); got != "userctl disable --now agent-bus-runner.service; unregister; helper remove state; remove template" {
		t.Fatalf("uninstall: %s", got)
	}
	other := done
	other.others = 1
	if got := strings.Join(runPlan(t, "uninstall", other, true), "; "); got != "userctl disable --now agent-bus-runner.service; unregister; helper remove state; helper remove agents" {
		t.Fatalf("uninstall --purge with another person's runner: %s", got)
	}
	for _, st := range []userState{
		{user: true, record: true, recordKind: "agent", recordOwner: "bob", seen: userSeen{State: "missing", Agents: "missing"}},
		{user: true, record: true, recordKind: "queue", recordOwner: "alice", seen: userSeen{State: "missing", Agents: "missing"}},
		{user: true, seen: userSeen{State: "other", Agents: "missing"}},
		{user: false, seen: userSeen{State: "missing", Agents: "missing"}},
	} {
		if _, err := userPlan("install", userRunner{person: "alice", home: "/home/alice", name: "#runner/alice-user@n"}, st, "u", false, &fakeUserOps{}); err == nil {
			t.Errorf("install accepted %+v", st)
		}
	}
}

func TestUserHelperStaysInsideTheHomeAndFollowsNoLink(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the helper refuses root; run as an ordinary user")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("AGENT_BUS_TOKEN=not-yours\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A planted state directory that is a link: observe must neither follow nor read it.
	state := filepath.Join(home, userStateRel)
	if err := os.MkdirAll(filepath.Dir(state), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(secret), state); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(secret, filepath.Join(filepath.Dir(secret), "runner.env")); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() error { return userHelper([]string{"observe"}) })
	var seen userSeen
	if err := json.Unmarshal(out, &seen); err != nil || seen.State != "other" || seen.Token != "" {
		t.Fatalf("observe through a link: %s %v", out, err)
	}
	if err := userHelper([]string{"remove", "state"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(secret), "runner.env")); err != nil {
		t.Fatalf("remove followed the link: %v", err)
	}
	// The real thing: directories 0700, runner.env 0600 with the token, observed back.
	if err := userHelper([]string{"mkdirs"}); err != nil {
		t.Fatal(err)
	}
	withStdin(t, "abc123\n", func() {
		if err := userHelper([]string{"write-env"}); err != nil {
			t.Fatal(err)
		}
	})
	fi, err := os.Stat(filepath.Join(state, "runner.env"))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("runner.env: %v %v", fi, err)
	}
	if d, _ := os.Stat(state); d.Mode().Perm() != 0o700 {
		t.Fatalf("state mode %v", d.Mode().Perm())
	}
	out = captureStdout(t, func() error { return userHelper([]string{"observe"}) })
	if err := json.Unmarshal(out, &seen); err != nil || seen != (userSeen{State: "dir", Agents: "dir", Token: "abc123"}) {
		t.Fatalf("observe: %s %v", out, err)
	}
}

func captureStdout(t *testing.T, f func() error) []byte {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	ferr := f()
	os.Stdout = old
	w.Close()
	if ferr != nil {
		t.Fatal(ferr)
	}
	b := make([]byte, 64<<10)
	n, _ := r.Read(b)
	return b[:n]
}

func withStdin(t *testing.T, in string, f func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.WriteString(in)
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()
	f()
}

func TestLingerNeedsARunningUserManager(t *testing.T) {
	oldL, oldR := lingerDir, runUser
	lingerDir, runUser = t.TempDir(), t.TempDir()
	t.Cleanup(func() { lingerDir, runUser = oldL, oldR })
	u := userRunner{person: "alice", uid: 1001}
	if lingerReady(u) {
		t.Fatal("no flag, no bus: ready")
	}
	os.WriteFile(filepath.Join(lingerDir, "alice"), nil, 0o644)
	if lingerReady(u) {
		t.Fatal("flag without a running manager counted as ready, so enable-linger would never run again")
	}
	os.MkdirAll(filepath.Join(runUser, "1001"), 0o700)
	os.WriteFile(filepath.Join(runUser, "1001", "bus"), nil, 0o600)
	if !lingerReady(u) {
		t.Fatal("flag and bus: not ready")
	}
}

func TestTheHelperBinaryMustBeReachableByThePerson(t *testing.T) {
	open := t.TempDir()
	os.Chmod(open, 0o755)
	os.Chmod(filepath.Dir(open), 0o755) // the test's own temp root is 0700
	exe := filepath.Join(open, "agent-bus-setup")
	os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755)
	os.Chmod(exe, 0o755)
	if err := reachableByAll(exe); err != nil {
		t.Fatalf("an open path refused: %v", err)
	}
	private := filepath.Join(t.TempDir(), "root-only")
	os.MkdirAll(private, 0o700)
	hidden := filepath.Join(private, "agent-bus-setup")
	os.WriteFile(hidden, []byte("#!/bin/sh\n"), 0o755)
	os.Chmod(private, 0o700)
	os.Chmod(filepath.Dir(private), 0o755)
	os.Chmod(hidden, 0o755)
	if err := reachableByAll(hidden); err == nil || !strings.Contains(err.Error(), "not searchable") {
		t.Fatalf("a setup under a 0700 directory: %v", err)
	}
	os.Chmod(exe, 0o750)
	if err := reachableByAll(exe); err == nil {
		t.Fatal("a setup not executable by others accepted")
	}
}

func TestTheHelperRefusesRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Setenv("HOME", "")
		if err := userHelper([]string{"observe"}); err == nil {
			t.Fatal("helper ran without a HOME")
		}
		return
	}
	if err := userHelper([]string{"observe"}); err == nil {
		t.Fatal("helper ran as root")
	}
}

// The installer's PATH has no say in which setpriv or systemctl root runs on a
// person's behalf: a directory it names that holds look-alikes is never run.
func TestUserCommandsIgnoreTheInstallersPATH(t *testing.T) {
	trusted, planted := t.TempDir(), t.TempDir()
	marker := filepath.Join(planted, "ran")
	write := func(dir, name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+text), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"setpriv", "systemctl", "loginctl", "getent"} {
		write(planted, name, "echo "+name+" >>"+marker+"\nexit 7\n")
	}
	write(trusted, "setpriv", "shift 5; exec \"$@\"\n")
	write(trusted, "systemctl", "[ \"$1\" = --version ] && echo 'systemd 245 (245)'\nexit 0\n")
	was := toolDirs
	toolDirs = []string{trusted}
	t.Cleanup(func() { toolDirs = was })
	t.Setenv("PATH", planted+":/usr/bin:/bin")
	u := userRunner{person: "alice", uid: 1001, gid: 1002, home: "/home/alice"}
	if err := (hostUserOps{}).userctl(u, "daemon-reload"); err != nil {
		t.Fatalf("the trusted tools were not the ones run: %v", err)
	}
	if argv := helperArgv("/usr/local/bin/agent-bus-setup", u, "observe"); argv[0] != filepath.Join(trusted, "setpriv") {
		t.Fatalf("the helper runs %q, want the trusted setpriv by absolute path", argv[0])
	}
	if err := userManagerAvailable(); err != nil {
		t.Fatalf("the availability check did not use the trusted systemctl: %v", err)
	}
	if got, err := os.ReadFile(marker); err == nil {
		t.Fatalf("PATH look-alikes ran: %q", got)
	}
}

// userManagerAvailable asks for a systemctl that answers and nothing more: no
// systemd version floor, since the person's manager is reached over its bus.
func TestUserManagerAvailableNeedsASystemctlThatAnswersAndNoVersion(t *testing.T) {
	dir := t.TempDir()
	was := toolDirs
	toolDirs = []string{dir}
	t.Cleanup(func() { toolDirs = was })
	if err := userManagerAvailable(); err == nil {
		t.Fatal("no systemctl was accepted")
	}
	for _, c := range []struct {
		say  string
		want bool
	}{
		{"systemd 219 (219)", true}, {"systemd 245 (245.4-4ubuntu3)", true}, {"systemd 257 (257.1)", true},
		{"upstart 1.13", false}, {"", false},
	} {
		if err := os.WriteFile(filepath.Join(dir, "systemctl"), []byte("#!/bin/sh\necho '"+c.say+"'\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := userManagerAvailable(); (err == nil) != c.want {
			t.Errorf("systemctl saying %q: error %v, want ok=%v", c.say, err, c.want)
		}
	}
}
