package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStartupFailureFixture(t *testing.T) {
	dir := os.Getenv("AB_STARTUP_FAILURE_FIXTURE")
	if dir == "" {
		return
	}
	mode := os.Getenv("AB_STARTUP_FAILURE_MODE")
	c := config{owner: "owner@h", db: filepath.Join(dir, "bus.db"), logDir: filepath.Join(dir, "logs"), addr: "127.0.0.1:0", sock: filepath.Join(dir, "run", "bus.sock")}
	switch mode {
	case "supervisor directory":
		c.create = true
		c.vouch = []string{"bad"}
	case "bus directory":
		c.create = true
		c.vouch = []string{"bad"}
	case "supervisor owner":
		c.create = true
		c.owner = "bad@@h"
	case "owner", "init owner":
		// Exercise main's role dispatch, not just runBus's direct check.
		os.Args = []string{os.Args[0], "-owner", "bad@@h", "-db", c.db, "-log-dir", c.logDir}
		os.Setenv(roleEnv, roleBus)
		if mode == "init owner" {
			os.Args = append(os.Args, "-init")
		}
		main()
		return
	case "inherited", "bad fd":
		c.create = true
	}
	if strings.HasPrefix(mode, "supervisor") && os.Getenv(roleEnv) != roleBus {
		runSupervisor(c)
	} else {
		runBus(c)
	}
}

func TestStartupFailuresAreReportedWithoutAChildCrashLoop(t *testing.T) {
	for _, tc := range []struct {
		mode, want string
		journal    bool
	}{
		{"supervisor directory", "--directory wants", false},
		{"supervisor owner", "owner:", false},
		{"init owner", "owner:", false},
		{"database", "database:", true},
		{"bus directory", "--directory wants", true},
		{"owner", "owner:", true},
		{"inherited", fdsEnv + " is empty", true},
		{"bad fd", "fd 3 (tcp)", true},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStartupFailureFixture$")
			child.Env = append(os.Environ(), "AB_STARTUP_FAILURE_FIXTURE="+dir, "AB_STARTUP_FAILURE_MODE="+tc.mode, accountsEnv+"={}", fdsEnv+"=")
			if tc.mode == "bad fd" {
				child.Env = append(child.Env, fdsEnv+"=tcp")
			}
			child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			child.Cancel = func() error { return syscall.Kill(-child.Process.Pid, syscall.SIGKILL) }
			output, err := child.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("startup did not refuse once; child kept restarting: %s", output)
			}
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("startup failure exit=%v, output=%s", err, output)
			}
			if !strings.Contains(string(output), tc.want) || strings.Contains(string(output), "bus died") {
				t.Fatalf("startup diagnostic: %s", output)
			}
			if tc.journal {
				data, err := os.ReadFile(filepath.Join(dir, "logs", "error.log"))
				if err != nil {
					t.Fatalf("startup failure did not reach journal: %v; stderr=%s", err, output)
				}
				if !strings.Contains(string(data), "the daemon cannot start:") || !strings.Contains(string(data), tc.want) {
					t.Fatalf("journal omitted startup cause: %s", data)
				}
			} else if _, err := os.Stat(filepath.Join(dir, "bus.db")); !os.IsNotExist(err) {
				t.Fatalf("invalid directory reached child database: %v", err)
			}
		})
	}
}
