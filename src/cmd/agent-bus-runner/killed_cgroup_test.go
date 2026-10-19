package main

import (
	"fmt"
	"github.com/parf/ai-agent-bus/internal/cgroup"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestForcedKeeperReloadUsesFreshCgroup(t *testing.T) {
	if os.Getenv("AB_TEST_KILLED_CGROUP") != "1" {
		// A private transient unit gives only this fixture a delegated tree. It
		// never writes a live runner's cgroups or signals a live process.
		cmd := exec.Command("systemd-run", "--user", "--quiet", "--wait", "--pipe", "--collect",
			"--unit=ab-killed-cgroup-test-"+fmt.Sprint(os.Getpid()), "--property=Delegate=yes",
			"--setenv=AB_TEST_KILLED_CGROUP=1", os.Args[0], "-test.run=^TestForcedKeeperReloadUsesFreshCgroup$", "-test.v")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("delegated keeper fixture: %v\n%s", err, out)
		}
		return
	}
	r, _, script := configurationFixture(t, "itself", `trap '' TERM; echo started >> "$AGENT_BUS_WORK/forced-starts"; while :; do sleep .02; done`, "on")
	r.mu.Lock()
	s := r.served["#fixture@h"]
	r.mu.Unlock()
	if s == nil {
		t.Fatal("sandboxed keeper was refused")
	}

	// Kill and immediately prepare, without waiting for the victim to exit.
	group, err := r.cg.Agent("immediate", cgroup.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	defer group.Remove()
	victim := exec.Command("/usr/bin/sleep", "30")
	victim.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: group.FD()}
	if err := victim.Start(); err != nil {
		t.Fatal(err)
	}
	if err := group.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := group.Prepare(); err != nil {
		t.Fatalf("immediate prepare: %v", err)
	}
	_ = victim.Wait()
	probe := exec.Command("/usr/bin/true")
	probe.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: group.FD()}
	if err := probe.Run(); err != nil {
		t.Fatalf("prepared group killed fresh child: %v", err)
	}
	path := filepath.Join(s.work, "forced-starts")
	await := func(want string) {
		t.Helper()
		end := time.Now().Add(4 * time.Second)
		for {
			raw, _ := os.ReadFile(path)
			if string(raw) == want {
				return
			}
			if time.Now().After(end) {
				t.Fatalf("forced-stop replacement never served: starts %q want %q", raw, want)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	await("started\n")
	if err := os.WriteFile(filepath.Join(filepath.Dir(script), "revision"), []byte("new package"), 0600); err != nil {
		t.Fatal(err)
	}
	r.reload()
	await("started\nstarted\n")
	// A stop and fresh adoption uses the same forced-stop path as retirement.
	if err := r.control("stop", "fixture"); err != nil {
		t.Fatal(err)
	}
	if err := r.control("start", "fixture"); err != nil {
		t.Fatal(err)
	}
	await("started\nstarted\nstarted\n")
}
