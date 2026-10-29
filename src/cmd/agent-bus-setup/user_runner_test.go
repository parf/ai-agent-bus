package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

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
	argv, err := userCtlArgv("alice", "enable", "--now", userUnit)
	if err != nil || strings.Join(argv, " ") != "systemctl --user -M alice@ enable --now agent-bus-runner.service" {
		t.Fatalf("%q %v", argv, err)
	}
	if _, err := userCtlArgv("alice", "daemon-reload"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]string{{"restart", "user@1001.service"}, {"stop", userUnit}, {"restart", userUnit},
		{"daemon-reexec"}, {"stop", "ab-session-work.scope"}, {"kill", userUnit}, {"try-restart", "ab-session-work.scope"},
		{"enable"}, {"try-restart", userUnit, "user@1001.service"}, {"enable", "--now", "ab-session-work.scope", userUnit}} {
		if _, err := userCtlArgv("alice", bad...); err == nil {
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
	if argv != "setpriv --reuid=1001 --regid=1002 --init-groups --no-new-privs -- /usr/local/bin/agent-bus-setup --user-runner-helper observe" {
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
