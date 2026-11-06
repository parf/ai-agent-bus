package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/parf/ai-agent-bus/internal/api"
)

func execLine(t *testing.T, unit string) string {
	t.Helper()
	for _, line := range strings.Split(unit, "\n") {
		if strings.HasPrefix(line, "ExecStart=") {
			return line
		}
	}
	t.Fatalf("no ExecStart in %q", unit)
	return ""
}

// A long flag a unit starts a program with is spelled with two dashes: the
// daemon and the runner refuse the single-dash form.
var singleDashLong = regexp.MustCompile(`^-[A-Za-z][A-Za-z-]+`)

func assertCanonical(t *testing.T, name, line string) {
	t.Helper()
	for _, field := range strings.Fields(strings.TrimPrefix(line, "ExecStart=")) {
		if singleDashLong.MatchString(field) && !strings.HasPrefix(field, "--") {
			t.Errorf("%s starts its program with %q, a single-dash long flag: %s", name, field, line)
		}
	}
}

// Every unit setup writes, whole ExecStart line asserted, with only canonical flags.
func TestEveryUnitSetupWritesUsesCanonicalFlags(t *testing.T) {
	daemon := execLine(t, unitFor("/prog/agent-busd", "127.0.0.1:6767", "owner@h", list{"alice=alice@h", "bob=bob@h"}, "/etc/agent-bus/tls"))
	wantDaemon := "ExecStart=/prog/agent-busd --addr 127.0.0.1:6767 --socket " + api.SystemSocket() +
		" --db " + svcHome + "/agent-bus.db --log-dir " + logDir + " --owner owner@h --syslog --user alice=alice@h --user bob=bob@h --tls-dir /etc/agent-bus/tls" +
		" --ssh-keys " + svcHome + "/.ssh/authorized_keys --ssh-token /prog/agent-bus-token"
	if daemon != wantDaemon {
		t.Errorf("the daemon unit starts:\n got %s\nwant %s", daemon, wantDaemon)
	}
	plain := execLine(t, unitFor("/prog/agent-busd", "127.0.0.1:6767", "owner@h", nil, ""))
	if strings.Contains(plain, "--user") || strings.Contains(plain, "--tls-dir") {
		t.Errorf("a node with no users and no TLS names neither: %s", plain)
	}
	runner := execLine(t, runnerTemplate("/prog/agent-bus-runner"))
	if want := "ExecStart=/prog/agent-bus-runner --agents " + sharedAgents + " --host " + stateRoot + "/%i-ab-runner"; runner != want {
		t.Errorf("the runner unit starts:\n got %s\nwant %s", runner, want)
	}
	user := execLine(t, userTemplate("/prog/agent-bus-runner"))
	if !regexp.MustCompile(`^ExecStart=/prog/agent-bus-runner --agents %h/\S+ --host %h/\S+$`).MatchString(user) {
		t.Errorf("the user runner unit starts: %s", user)
	}
	for name, line := range map[string]string{"daemon": daemon, "daemon, plain": plain, "runner": runner, "user runner": user} {
		assertCanonical(t, name, line)
	}
	// The units the repository ships beside setup: the web face and the maintenance timers.
	for _, path := range []string{"../../web/agent-bus-web.service", "../../maintenance/agent-bus-cache-cleanup.service", "../../maintenance/agent-bus-node-cleanup.service"} {
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		assertCanonical(t, filepath.Base(path), execLine(t, string(text)))
	}
}

// A unit an older release wrote, with single-dash flags, is not the one setup
// writes now, so a re-run writes it again and restarts the running daemon (or
// the runner) onto it, through the same unit comparison every run makes.
func TestAnOldSingleDashUnitIsRewrittenOnTheNextRun(t *testing.T) {
	old := func(unit string) string {
		return regexp.MustCompile(` --([a-z])`).ReplaceAllString(unit, " -$1")
	}
	unit := unitFor("/prog/agent-busd", "127.0.0.1:6767", "owner@h", list{"alice=alice@h"}, "/etc/agent-bus/tls")
	stale := old(unit)
	if !strings.Contains(stale, "ExecStart=/prog/agent-busd -addr ") {
		t.Fatalf("the fixture is not a single-dash unit: %s", execLine(t, stale))
	}
	if !unitChanged([]byte(stale), true, unit) {
		t.Error("an old single-dash daemon unit is not seen as changed")
	}
	if unitChanged([]byte(unit), true, unit) {
		t.Error("an unchanged daemon unit is seen as changed")
	}
	if !unitChanged(nil, false, unit) {
		t.Error("a missing unit is not seen as changed")
	}
	// The root-account runner: the unit diff writes the template and restarts the running runner.
	p, err := runnerFor("alice", "h")
	if err != nil {
		t.Fatal(err)
	}
	current := runnerTemplate("/prog/agent-bus-runner")
	plan, err := runnerPlan("install", p, installedState(p, old(current)), current, false, runnerOps{})
	if err != nil {
		t.Fatal(err)
	}
	if got := says(t, plan); !strings.Contains(got, "write "+runnerTemplatePath+" and reload systemd") || !strings.Contains(got, "restart agent-bus-runner@alice.service") {
		t.Errorf("an old single-dash runner unit was not rewritten and restarted:\n%s", got)
	}
	// The user-account runner: the template is written again.
	stale2 := userState{user: true, seen: userSeen{State: "dir", Agents: "dir", Token: "t"}, unit: old(userTemplate("/r")),
		record: true, recordKind: "agent", recordOwner: "alice", tokenValid: true, linger: true, enabled: true, active: true, files: true}
	got := strings.Join(runPlan(t, "install", stale2, false), "; ")
	if !strings.Contains(got, "template") {
		t.Errorf("an old single-dash user runner unit was not rewritten: %s", got)
	}
}
