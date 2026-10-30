package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSupervisorPIDComesFromSpawner(t *testing.T) {
	k := &child{env: []string{supervisorPIDEnv + "=3"}}
	got := k.environ([]string{"PATH=/usr/bin", supervisorPIDEnv + "=2"})
	var pids []string
	for _, entry := range got {
		if strings.HasPrefix(entry, supervisorPIDEnv+"=") {
			pids = append(pids, entry)
		}
	}
	want := supervisorPIDEnv + "=" + strconv.Itoa(os.Getpid())
	if len(pids) != 1 || pids[0] != want {
		t.Fatalf("child supervisor PID %v, want only %s", pids, want)
	}
}

func TestParentWatchRefusesMissingOrInvalidSupervisor(t *testing.T) {
	for _, value := range []string{"", "not-a-pid", "0", "1", "-1"} {
		t.Setenv(supervisorPIDEnv, value)
		if err := watchSupervisorParent(); err == nil {
			t.Fatalf("accepted invalid supervisor PID %q", value)
		}
	}
}

// Exercise the actual fallback even on Linux, without the Linux kernel death
// signal. The inherited listener makes an orphan observable even if it is
// reparented to launchd or a subreaper before the watcher starts.
func TestParentWatchReleasesListenersBeforeAndAfterStartup(t *testing.T) {
	for _, early := range []bool{true, false} {
		t.Run(fmt.Sprintf("supervisor-dies-before-watch=%v", early), func(t *testing.T) {
			root := t.TempDir()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			address := listener.Addr().String()
			fd, err := listener.(*net.TCPListener).File()
			listener.Close()
			if err != nil {
				t.Fatal(err)
			}
			defer fd.Close()
			log, err := os.Create(filepath.Join(root, "log"))
			if err != nil {
				t.Fatal(err)
			}
			defer log.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestParentWatchFixture$")
			cmd.Env = append(os.Environ(), "AB_PARENT_FIXTURE=launcher", "AB_PARENT_ROOT="+root,
				fmt.Sprintf("AB_PARENT_EARLY=%v", early))
			cmd.ExtraFiles = []*os.File{fd}
			cmd.Stdout, cmd.Stderr = log, log
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			waited := false
			defer func() {
				if !waited {
					cmd.Process.Kill()
					cmd.Wait()
				}
			}()
			waitFile := func(name string) []byte {
				t.Helper()
				for until := time.Now().Add(3 * time.Second); time.Now().Before(until); time.Sleep(10 * time.Millisecond) {
					if raw, err := os.ReadFile(filepath.Join(root, name)); err == nil {
						return raw
					}
				}
				raw, _ := os.ReadFile(log.Name())
				t.Fatalf("fixture did not reach %s: %s", name, raw)
				return nil
			}
			pid, err := strconv.Atoi(string(waitFile("pid")))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				p, _ := os.FindProcess(pid)
				if p != nil {
					p.Kill()
				}
			}()
			fd.Close() // only the fixture's copies can keep the port bound now
			if early {
				if err := cmd.Wait(); err != nil {
					t.Fatal(err)
				}
				waited = true
			}
			if err := os.WriteFile(filepath.Join(root, "start-watch"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			if !early {
				waitFile("watching")
				cmd.Process.Kill()
				cmd.Wait()
				waited = true
			}
			for until := time.Now().Add(2200 * time.Millisecond); ; time.Sleep(10 * time.Millisecond) {
				if next, err := net.Listen("tcp", address); err == nil {
					next.Close()
					break
				}
				if time.Now().After(until) {
					t.Fatalf("orphan survived watchParent startup/death and retained inherited listener (early=%v)", early)
				}
			}
			if early {
				waitFile("refused")
			}
		})
	}
}

func TestParentWatchFixture(t *testing.T) {
	mode, root := os.Getenv("AB_PARENT_FIXTURE"), os.Getenv("AB_PARENT_ROOT")
	if mode == "" {
		return
	}
	if mode == "launcher" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestParentWatchFixture$")
		cmd.Env = (&child{env: []string{"AB_PARENT_FIXTURE=child"}}).environ(os.Environ())
		cmd.ExtraFiles = []*os.File{os.NewFile(3, "inherited-listener")}
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "pid"), []byte(strconv.Itoa(cmd.Process.Pid)), 0600); err != nil {
			t.Fatal(err)
		}
		if os.Getenv("AB_PARENT_EARLY") != "true" {
			time.Sleep(6 * time.Second)
		}
		os.Exit(0)
	}
	for until := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		if _, err := os.Stat(filepath.Join(root, "start-watch")); err == nil {
			break
		}
		if time.Now().After(until) {
			os.Exit(72)
		}
	}
	if err := watchSupervisorParent(); err != nil {
		os.WriteFile(filepath.Join(root, "refused"), []byte(err.Error()), 0600)
		os.Exit(0)
	}
	os.WriteFile(filepath.Join(root, "watching"), nil, 0600)
	time.Sleep(6 * time.Second) // bounded even if a mutant disables the watcher
	os.Exit(73)
}
