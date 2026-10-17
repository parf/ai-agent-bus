package main

import (
	"strings"
	"testing"
)

// The host as an install leaves it: every piece present and right.
func installedState(p personRunner, unit string) runnerState {
	return runnerState{
		user: true, account: true, personAccount: true,
		root:    dirState{true, "root", 0o755},
		home:    dirState{true, "root", 0o755},
		agents:  dirState{true, p.person, 0o755},
		private: dirState{true, p.account, 0o700},
		unit:    unit, record: true, envValid: true, enabled: true, active: true,
	}
}

func says(t *testing.T, plan []action) string {
	t.Helper()
	var out []string
	for _, a := range plan {
		out = append(out, a.say)
	}
	return strings.Join(out, "\n")
}

// Each verb leaves the host in its stated state, and a second run changes
// nothing (runner wave M.11).
func TestEachRunnerVerbPlansOnlyWhatIsMissing(t *testing.T) {
	p, err := runnerFor("alice", "h")
	if err != nil {
		t.Fatal(err)
	}
	unit := runnerTemplate("/usr/bin/agent-bus-runner")

	// A fresh host: everything is planned, ending in the start.
	fresh, err := runnerPlan("install", p, runnerState{user: true, personAccount: true}, unit, false, runnerOps{})
	if err != nil {
		t.Fatal(err)
	}
	got := says(t, fresh)
	for _, want := range []string{
		"create the system account alice-ab-runner",
		"make /var/lib/agent-bus/runners/alice/agent.d alice's own, 0755",
		"make /var/lib/agent-bus/runners/alice/runner alice-ab-runner's own, 0700",
		"write " + runnerTemplatePath,
		"create #runner/alice@h, alice's agent, allowing @owner",
		"write /var/lib/agent-bus/runners/alice/runner/runner.env with #runner/alice@h's token, alice-ab-runner's alone",
		"enable and start agent-bus-runner@alice.service",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("a fresh install does not %q:\n%s", want, got)
		}
	}

	// Installed: a second install plans nothing at all.
	done := installedState(p, unit)
	if again, err := runnerPlan("install", p, done, unit, false, runnerOps{}); err != nil || len(again) != 0 {
		t.Fatalf("a second install planned %q, %v", says(t, again), err)
	}
	// A token gone stale is written again and the running runner restarted.
	stale := done
	stale.envValid = false
	if got := says(t, mustPlan(t, "install", p, stale, unit, false)); !strings.Contains(got, "runner.env") || !strings.Contains(got, "restart agent-bus-runner@alice.service") {
		t.Errorf("a stale token planned:\n%s", got)
	}

	// Disable stops and disables, and keeps every file; then plans nothing.
	if got := says(t, mustPlan(t, "disable", p, done, unit, false)); got != "stop and disable agent-bus-runner@alice.service, keeping its files" {
		t.Errorf("disable planned:\n%s", got)
	}
	off := done
	off.enabled, off.active = false, false
	if again := mustPlan(t, "disable", p, off, unit, false); len(again) != 0 {
		t.Errorf("a second disable planned %q", says(t, again))
	}
	// Enable brings it back, and refuses a runner that is not installed.
	if got := says(t, mustPlan(t, "enable", p, off, unit, false)); got != "enable and start agent-bus-runner@alice.service" {
		t.Errorf("enable planned:\n%s", got)
	}
	if _, err := runnerPlan("enable", p, runnerState{user: true}, unit, false, runnerOps{}); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("enable on a bare host: %v", err)
	}

	// Uninstall leaves no account, unit or record behind, and keeps agent.d.
	gone := says(t, mustPlan(t, "uninstall", p, done, unit, false))
	for _, want := range []string{
		"stop and disable agent-bus-runner@alice.service",
		"remove #runner/alice@h from the bus",
		"remove /var/lib/agent-bus/runners/alice/runner and keep /var/lib/agent-bus/runners/alice/agent.d",
		"delete the system account alice-ab-runner",
		"remove " + runnerTemplatePath + ", the last runner's",
	} {
		if !strings.Contains(gone, want) {
			t.Errorf("uninstall does not %q:\n%s", want, gone)
		}
	}
	// With another person's runner on the host, the template stays.
	shared := done
	shared.others = 1
	if got := says(t, mustPlan(t, "uninstall", p, shared, unit, false)); strings.Contains(got, runnerTemplatePath) {
		t.Errorf("uninstall beside another runner removes the template:\n%s", got)
	}
	// --purge takes agent.d too.
	if got := says(t, mustPlan(t, "uninstall", p, done, unit, true)); !strings.Contains(got, "remove /var/lib/agent-bus/runners/alice, agent.d included") {
		t.Errorf("uninstall --purge planned:\n%s", got)
	}
	// Uninstalled: a second uninstall plans nothing.
	if again := mustPlan(t, "uninstall", p, runnerState{user: true, root: done.root, home: done.home, agents: done.agents}, unit, false); len(again) != 0 {
		t.Errorf("a second uninstall planned %q", says(t, again))
	}
}

func mustPlan(t *testing.T, verb string, p personRunner, st runnerState, unit string, purge bool) []action {
	t.Helper()
	plan, err := runnerPlan(verb, p, st, unit, purge, runnerOps{})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

// A runner is a person's: a name that is not an active bus User is refused,
// and so is one that cannot be part of an account's name.
func TestARunnerIsOnlyForABusUser(t *testing.T) {
	p, _ := runnerFor("ghost", "h")
	if _, err := runnerPlan("install", p, runnerState{}, "", false, runnerOps{}); err == nil || !strings.Contains(err.Error(), "ghost is not an active bus User") {
		t.Fatalf("installing for a non-User: %v", err)
	}
	for _, bad := range []string{"Alice", "alice@github", "a/b", "", strings.Repeat("a", 23)} {
		if _, err := runnerFor(bad, "h"); err == nil {
			t.Errorf("runnerFor(%q) was accepted", bad)
		}
	}
	if _, err := runnerPlan("frobnicate", p, runnerState{user: true}, "", false, runnerOps{}); err == nil {
		t.Error("an unknown verb was planned")
	}
}

// The template runs each instance as its own person's account and gives it
// that person's token, starting nothing while there is none.
func TestTheRunnerTemplateIsEachPersonsOwn(t *testing.T) {
	u := runnerTemplate("/usr/bin/agent-bus-runner")
	for _, want := range []string{
		"User=%i-ab-runner\n",
		"EnvironmentFile=/var/lib/agent-bus/runners/%i/runner/runner.env\n",
		"ConditionPathExists=/var/lib/agent-bus/runners/%i/runner/runner.env\n",
		"ExecStart=/usr/bin/agent-bus-runner --home /var/lib/agent-bus/runners/%i\n",
		"ReadWritePaths=/var/lib/agent-bus/runners/%i/runner\n",
		"Delegate=yes\n", "NoNewPrivileges=yes\n", "Wants=agent-busd.service\n",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("the template lacks %q", strings.TrimSpace(want))
		}
	}
	for _, never := range []string{"Requires=", "AmbientCapabilities"} {
		if strings.Contains(u, never) {
			t.Errorf("the template has %q", never)
		}
	}
}
