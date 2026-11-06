package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func execStart(unit string) string {
	for _, line := range strings.Split(unit, "\n") {
		if strings.HasPrefix(line, "ExecStart=") {
			return line
		}
	}
	return ""
}

// A managed release is started through installRoot/current, so an upgrade moves
// every runner and a pruned release leaves none dangling; a checkout setup
// starts the runner beside itself.
func TestRunnerUnitStartsTheCurrentReleaseNotTheOneSetupRanFrom(t *testing.T) {
	root := t.TempDir()
	saved := installRoot
	installRoot = root
	t.Cleanup(func() { installRoot = saved })
	const tail = " --agents /var/lib/agent-bus/agent.d --host /var/lib/agent-bus/%i-ab-runner"
	for _, c := range []struct{ name, self, want string }{
		{"release r1", filepath.Join(root, "releases", "r1", "agent-bus-setup"), "ExecStart=" + filepath.Join(root, "current", "agent-bus-runner") + tail},
		{"release r2", filepath.Join(root, "releases", "r2", "agent-bus-setup"), "ExecStart=" + filepath.Join(root, "current", "agent-bus-runner") + tail},
		{"checkout", "/home/dev/src/ai-agent-bus/tmp/build/agent-bus-setup", "ExecStart=/home/dev/src/ai-agent-bus/tmp/build/agent-bus-runner" + tail},
		{"a lookalike directory", filepath.Join(root, "releases-old", "r1", "agent-bus-setup"), "ExecStart=" + filepath.Join(root, "releases-old", "r1", "agent-bus-runner") + tail},
	} {
		if got := execStart(runnerTemplate(runnerExecutable(c.self))); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	r1 := runnerTemplate(runnerExecutable(filepath.Join(root, "releases", "r1", "agent-bus-setup")))
	r2 := runnerTemplate(runnerExecutable(filepath.Join(root, "releases", "r2", "agent-bus-setup")))
	if r1 != r2 {
		t.Errorf("an upgrade changes the unit text, so it would have to rewrite every runner:\n%s\n%s", r1, r2)
	}
}

// An old unit that pins one release is rewritten by the next install through the
// ordinary unit diff, and the running runner restarted; then nothing changes.
func TestInstallRewritesAnOldPinnedUnit(t *testing.T) {
	root := t.TempDir()
	saved := installRoot
	installRoot = root
	t.Cleanup(func() { installRoot = saved })
	p, err := runnerFor("alice", "h")
	if err != nil {
		t.Fatal(err)
	}
	pinned := runnerTemplate(filepath.Join(root, "releases", "r1", "agent-bus-runner"))
	current := runnerTemplate(runnerExecutable(filepath.Join(root, "releases", "r2", "agent-bus-setup")))
	plan, err := runnerPlan("install", p, installedState(p, pinned), current, false, runnerOps{})
	if err != nil {
		t.Fatal(err)
	}
	got := says(t, plan)
	if !strings.Contains(got, "write "+runnerTemplatePath+" and reload systemd") || !strings.Contains(got, "restart agent-bus-runner@alice.service") {
		t.Errorf("an old pinned unit was not rewritten and restarted:\n%s", got)
	}
	if again, err := runnerPlan("install", p, installedState(p, current), current, false, runnerOps{}); err != nil || len(again) != 0 {
		t.Errorf("a second install planned %q, %v", says(t, again), err)
	}
}

// A managed release with no current link fails the install; nothing points at nothing.
func TestInstallRefusesAReleaseWithNoCurrentRunner(t *testing.T) {
	root := t.TempDir()
	saved := installRoot
	installRoot = root
	t.Cleanup(func() { installRoot = saved })
	release := filepath.Join(root, "releases", "r1")
	if err := os.MkdirAll(release, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(release, "agent-bus-runner"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	self := filepath.Join(release, "agent-bus-setup")
	if exe, err := runnerProgram(self, "install"); err == nil || !strings.Contains(err.Error(), filepath.Join(root, "current", "agent-bus-runner")) {
		t.Fatalf("no current link: %q, %v; want the missing path named", exe, err)
	}
	if err := os.Symlink(release, filepath.Join(root, "current")); err != nil {
		t.Fatal(err)
	}
	if exe, err := runnerProgram(self, "install"); err != nil || exe != filepath.Join(root, "current", "agent-bus-runner") {
		t.Fatalf("with a current link: %q, %v", exe, err)
	}
	// Other verbs do not need the program.
	os.Remove(filepath.Join(root, "current"))
	if _, err := runnerProgram(self, "disable"); err != nil {
		t.Errorf("disable needs no runner program: %v", err)
	}
}
