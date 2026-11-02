package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A shard that starts late is a loaded host, not a failure: the wait is on the
// shard's own start signal, bounded generously, and ends at once if the driver
// itself exits (J69).
const (
	smokeStartWithin = 30 * time.Second
	smokeExitWithin  = 15 * time.Second
)

// Exercise the real driver's traps with one harmless shard and no build.
func TestSmokeInterruptStopsItsTimeoutShard(t *testing.T) { smokeInterrupt(t, 0) }

// The same, with the shard starting later than the old fixed three-second wait.
func TestSmokeInterruptWaitsForALateShard(t *testing.T) { smokeInterrupt(t, 3500*time.Millisecond) }

// The real go_unit shard exports slow mode but consumes its child stamps.
// A fixture driver must be isolated from both that mode and any future stamp.
func TestSmokeInterruptUnderInheritedSlowEnvironment(t *testing.T) {
	t.Setenv("MODE", "slow")
	t.Setenv("TARGETS", "A,C,E,H")
	t.Setenv("SMOKE_PLAN", "/nonexistent/j73-parent-plan")
	t.Setenv("SMOKE_CHILD_SHARD", "j73-must-not-survive")
	t.Setenv("SMOKE_CHILD_PLAN", "/nonexistent/j73-parent-plan")
	t.Setenv("SMOKE_DRIVER_CHILD", "1")
	smokeInterrupt(t, 0)
}

func smokeInterrupt(t *testing.T, startDelay time.Duration) {
	source, err := os.ReadFile("../../smoke.sh")
	if err != nil {
		t.Fatal(err)
	}
	driver := strings.Split(string(source), "\n# A shard:")[0]
	driver = regexp.MustCompile(`(?s)SHARDS=\(.*?\)`).ReplaceAllString(driver, "SHARDS=(fixture)")
	driver = strings.Replace(driver, `python3 ./smoke-select.py --mode "$MODE" --targets "$TARGETS" --out "$SMOKE_PLAN" || exit 2`, `printf "fixture\tnormal\n" >"$SMOKE_PLAN"`, 1)
	driver = strings.Replace(driver, `bash ./build.sh "$B/bin" || exit 1`, `mkdir -p "$B/bin"`, 1)
	driver = strings.Replace(driver, `if [ -z "$SHARD" ]; then`, `if [ -n "$SHARD" ]; then sleep `+strconv.FormatFloat(startDelay.Seconds(), 'f', 3, 64)+`; echo "$$" >"$PROBE_PID"; exec sleep 30; fi`+"\n"+`if [ -z "$SHARD" ]; then`, 1)
	dir := t.TempDir()
	script, pidfile := filepath.Join(dir, "driver.sh"), filepath.Join(dir, "child.pid")
	// The driver sources these beside itself.
	for _, name := range []string{"smoke-options.sh", "smoke-port.sh", "smoke-gofail.sh", "smoke-bun.sh", "clients/cargo-target.sh"} {
		body, err := os.ReadFile("../../" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(script, []byte(driver), 0700); err != nil {
		t.Fatal(err)
	}
	out, err := os.Create(filepath.Join(dir, "output"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	// This stub belongs only to this fixture's PATH. A mode leak cannot reach
	// the real bus, and its calls are counted even if its fake lock would allow it.
	stubDir := filepath.Join(dir, "stub-bin")
	if err := os.Mkdir(stubDir, 0700); err != nil {
		t.Fatal(err)
	}
	lockCalls := filepath.Join(dir, "lock.calls")
	stub := `#!/bin/bash
printf '%s\n' "$1" >>"$PROBE_LOCK_CALLS"
case "$1" in
 status) printf '%s\n' '{"you":"fixture@j73"}' ;;
 holders) printf '%s\n' 'smoke-gate@parf.us slow fixture@j73' ;;
 *) exit 91 ;;
esac
`
	if err := os.WriteFile(filepath.Join(stubDir, "agent-bus"), []byte(stub), 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script)
	// Its port claim locks in the test's own directory, never the host's.
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name == "MODE" || name == "TARGETS" || name == "PORT" || name == "PATH" || strings.HasPrefix(name, "SMOKE_") || strings.HasPrefix(name, "AGENT_BUS_") {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"), "PROBE_LOCK_CALLS="+lockCalls, "PROBE_PID="+pidfile, "SMOKE_CPU_PERCENT=0", "TMPDIR="+dir, "SMOKE_PORT_LOCKS="+filepath.Join(dir, "ports"))
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	child, group := 0, 0
	defer func() {
		if group > 0 {
			_ = syscall.Kill(-group, syscall.SIGKILL)
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}()
	until := time.Now().Add(smokeStartWithin)
	for time.Now().Before(until) {
		select {
		case err := <-exited:
			text, _ := os.ReadFile(filepath.Join(dir, "output"))
			t.Fatalf("the driver exited (%v) before its fixture shard started: %s", err, text)
		default:
		}
		if b, err := os.ReadFile(pidfile); err == nil {
			child, err = strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil {
				t.Fatal(err)
			}
			if child > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if child == 0 {
		t.Fatal("the fixture shard did not start")
	}
	group, err = syscall.Getpgid(child)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(smokeExitWithin):
		t.Fatal("interrupted smoke driver did not exit")
	}
	if calls, err := os.ReadFile(lockCalls); err == nil && len(calls) > 0 {
		t.Fatalf("nested driver inherited gate state and consulted the lock: %s", calls)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join(dir, "output"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(text), "No such file or directory") {
		t.Fatalf("fixture driver omitted a sourced helper: %s", text)
	}
	if err := syscall.Kill(child, 0); err == nil {
		t.Fatal("interrupted smoke left its timeout shard running")
	}
}
