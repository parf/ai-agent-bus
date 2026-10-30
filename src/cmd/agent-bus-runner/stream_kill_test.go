package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/cgroup"
)

// A process past its deadline is killed with what it started, and the pool's
// other processes, in the same cgroup, keep what they hold. The cgroup needs a
// delegated subtree, so the check re-runs itself in a transient user unit that
// has one (the helper process below), and skips where there is no such unit.
func TestJsonlDeadlineKillSparesOtherProcessesInThePool(t *testing.T) {
	if os.Getenv("AB_STREAM_KILL_HELPER") != "1" {
		if _, err := exec.LookPath("systemd-run"); err != nil {
			t.Skip("needs systemd-run for a delegated cgroup")
		}
		c := exec.Command("systemd-run", "--user", "--quiet", "--wait", "--pipe", "--collect", "--unit=ab-stream-kill-"+fmt.Sprint(os.Getpid()),
			"--property=Delegate=yes", "--setenv=AB_STREAM_KILL_HELPER=1", os.Args[0], "-test.run=^TestJsonlDeadlineKillSparesOtherProcessesInThePool$", "-test.v")
		out, err := c.CombinedOutput()
		switch verdict := delegatedVerdict(out, err); verdict {
		case "skip":
			t.Skipf("no delegated user unit here: %v\n%s", err, out)
		case "pass":
		default:
			t.Fatalf("the check failed in its delegated unit (%s):\n%s", verdict, out)
		}
		return
	}
	tree, err := cgroup.Delegated()
	if err != nil {
		t.Fatal(err)
	}
	group, err := tree.Agent("stream-kill", cgroup.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	defer group.Remove()
	defer group.Kill()
	start := func() (*exec.Cmd, chan error) {
		c := exec.Command("/bin/sh", "-c", "sleep 30 & wait")
		c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, UseCgroupFD: true, CgroupFD: group.FD()}
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- c.Wait() }()
		return c, done
	}
	expired, done := start()
	_, sibling := start()
	s := &served{jail: &jail{group: group}}
	st := &stream{r: &runner{}, s: s}
	st.kill(&streamChild{cmd: expired})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("the expired process did not exit")
	}
	select {
	case err := <-sibling:
		t.Fatalf("the deadline kill also killed a different pool process: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
}

// delegatedVerdict reads what the transient unit printed: only a unit that
// never started the test may skip; a test that started and crashed, or ended
// without passing, fails.
func delegatedVerdict(out []byte, err error) string {
	started := bytes.Contains(out, []byte("=== RUN"))
	switch {
	case !started && err != nil && unavailable(out):
		return "skip"
	case err == nil && bytes.Contains(out, []byte("--- PASS")) && !bytes.Contains(out, []byte("--- FAIL")):
		return "pass"
	case err != nil:
		return "exit failed"
	}
	return "no pass reported"
}

// unavailable is systemd-run saying there is no user manager to ask for a
// delegated unit, which is the one reason to skip.
func unavailable(out []byte) bool {
	for _, say := range []string{"Failed to connect to bus", "Failed to connect to user scope bus", "Failed to create bus connection", "Interactive authentication required", "Access denied"} {
		if bytes.Contains(out, []byte(say)) {
			return true
		}
	}
	return false
}

func TestDelegatedVerdictSkipsOnlyWhenTheUnitNeverRanTheTest(t *testing.T) {
	boom := errors.New("exit status 2")
	for _, tc := range []struct {
		out  string
		err  error
		want string
	}{
		{"Failed to connect to bus: No medium found\n", boom, "skip"},
		{"Failed to start transient service unit: Unit ab-stream-kill-123.service already exists.\n", boom, "exit failed"},
		{"Failed to start transient service unit: Interactive authentication required.\n", boom, "skip"},
		{"panic: boom\n", boom, "exit failed"}, // a helper that crashes before its test starts
		{"", boom, "exit failed"},
		{"fork/exec: no such file or directory\n", boom, "exit failed"},
		{"=== RUN   T\npanic: boom\n", boom, "exit failed"},
		{"=== RUN   T\n--- FAIL: T\n", boom, "exit failed"},
		{"=== RUN   T\n--- PASS: T\n", nil, "pass"},
		{"", nil, "no pass reported"},
		{"=== RUN   T\n", nil, "no pass reported"},
	} {
		if got := delegatedVerdict([]byte(tc.out), tc.err); got != tc.want {
			t.Errorf("output %q err %v: %q, want %q", tc.out, tc.err, got, tc.want)
		}
	}
}
