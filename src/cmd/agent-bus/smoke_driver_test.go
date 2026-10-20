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

// Exercise the real driver's traps with one harmless shard and no build.
func TestSmokeInterruptStopsItsTimeoutShard(t *testing.T) {
	source, err := os.ReadFile("../../smoke.sh")
	if err != nil {
		t.Fatal(err)
	}
	driver := strings.Split(string(source), "\n# A shard:")[0]
	driver = regexp.MustCompile(`(?s)SHARDS=\(.*?\)`).ReplaceAllString(driver, "SHARDS=(fixture)")
	driver = strings.Replace(driver, `bash ./build.sh "$B/bin" || exit 1`, `mkdir -p "$B/bin"`, 1)
	driver = strings.Replace(driver, `if [ -z "$SHARD" ]; then`, `if [ -n "$SHARD" ]; then echo "$$" >"$PROBE_PID"; exec sleep 30; fi`+"\n"+`if [ -z "$SHARD" ]; then`, 1)
	dir := t.TempDir()
	script, pidfile := filepath.Join(dir, "driver.sh"), filepath.Join(dir, "child.pid")
	options, err := os.ReadFile("../../smoke-options.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "smoke-options.sh"), options, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte(driver), 0700); err != nil {
		t.Fatal(err)
	}
	out, err := os.Create(filepath.Join(dir, "output"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(), "PROBE_PID="+pidfile, "TMPDIR="+dir)
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
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
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
	case <-time.After(3 * time.Second):
		t.Fatal("interrupted smoke driver did not exit")
	}
	if err := syscall.Kill(child, 0); err == nil {
		t.Fatal("interrupted smoke left its timeout shard running")
	}
}
