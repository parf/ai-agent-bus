package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/parf/ai-agent-bus/internal/cgroup"
)

// J30b: an agent's processes share its cgroup, so stopping one for a replaced
// socket must reach only that process's own tree, never a sibling that has
// already started again on the new socket.
func TestSignalOneSparesASiblingInTheSharedCgroup(t *testing.T) {
	if os.Getenv("AB_TEST_SIGNAL_ONE") != "1" {
		// A private transient unit gives only this fixture a delegated tree. It
		// never writes a live runner's cgroups or signals a live process.
		cmd := exec.Command("systemd-run", "--user", "--quiet", "--wait", "--pipe", "--collect",
			"--unit=ab-signal-one-test-"+fmt.Sprint(os.Getpid()), "--property=Delegate=yes",
			"--setenv=AB_TEST_SIGNAL_ONE=1", os.Args[0], "-test.run=^TestSignalOneSparesASiblingInTheSharedCgroup$", "-test.v")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("delegated fixture: %v\n%s", err, out)
		}
		return
	}
	tree, err := cgroup.Delegated()
	if err != nil {
		t.Fatal(err)
	}
	group, err := tree.Agent("signal-one", cgroup.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	defer group.Remove()
	start := func() *exec.Cmd {
		// A shell with a child, as a sandboxed process has one under bubblewrap.
		c := exec.Command("/bin/sh", "-c", "sleep 30 & wait")
		c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, UseCgroupFD: true, CgroupFD: group.FD()}
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
		return c
	}
	stopping, sibling := start(), start()
	defer group.Kill()
	exited := func(c *exec.Cmd) chan error {
		ch := make(chan error, 1)
		go func() { ch <- c.Wait() }()
		return ch
	}
	stoppingDone, siblingDone := exited(stopping), exited(sibling)
	k := &keeper{}
	j := &jail{group: group}
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL} {
		k.signalOne(stopping, j, sig)
	}
	select {
	case <-stoppingDone:
	case <-time.After(2 * time.Second):
		t.Fatal("the process asked to stop is still running")
	}
	select {
	case err := <-siblingDone:
		t.Fatalf("the sibling in the same cgroup was signalled too: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
}
