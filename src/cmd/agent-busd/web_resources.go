package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// These limits cover the wrapper and all web descendants, not the bus.
// See docs/11-processes.md#web-resource-limits.
var webLimits = []struct{ file, value string }{
	{"memory.max", "268435456"}, // 256 MiB
	{"memory.swap.max", "0"},
	{"memory.oom.group", "1"},
	{"pids.max", "64"},
	{"cpu.max", "100000 100000"}, // one CPU worth of time
}

type webResources struct {
	dir string
	fd  *os.File
}

// Only the explicit systemd layout is accepted. Never walk upward looking for
// something writable: a containing slice can belong to a different authority.
func webResourceRoot() (string, error) {
	b, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(b))
	if !strings.HasPrefix(line, "0::/") || strings.Contains(line, "\n") {
		return "", errors.New("unified cgroup v2 is required")
	}
	path := strings.TrimPrefix(line, "0::")
	if filepath.Base(path) != "supervisor" {
		return "", errors.New("run in a systemd unit with Delegate=cpu memory pids and DelegateSubgroup=supervisor")
	}
	root := filepath.Join("/sys/fs/cgroup", filepath.Dir(path))
	var fs syscall.Statfs_t
	if err := syscall.Statfs(root, &fs); err != nil || fs.Type != 0x63677270 {
		return "", errors.New("delegation must be on cgroup v2")
	}
	var stat syscall.Stat_t
	if err := syscall.Stat(root, &stat); err != nil || stat.Uid != uint32(os.Geteuid()) {
		return "", errors.New("delegated cgroup must belong to the supervisor account")
	}
	marker := make([]byte, 8)
	n, markerErr := syscall.Getxattr(root, "user.delegate", marker)
	marked := markerErr == nil && string(marker[:n]) == "1"
	// The system manager marks a delegated service with user.delegate. A user
	// manager cannot add that xattr to its own transient unit, so development
	// launches need an explicit opt-in on that unit. Ownership, controllers,
	// the named subgroup and writability are still verified independently.
	if !marked && os.Getenv("AGENT_BUS_WEB_USER_DELEGATION") != "1" {
		return "", errors.New("parent cgroup is not system-delegated; a transient user unit must set AGENT_BUS_WEB_USER_DELEGATION=1")
	}
	writable, err := os.OpenFile(filepath.Join(root, "cgroup.subtree_control"), os.O_WRONLY, 0)
	if err != nil {
		return "", errors.New("delegated cgroup is not writable")
	}
	if err := writable.Close(); err != nil {
		return "", err
	}
	b, err = os.ReadFile(filepath.Join(root, "cgroup.controllers"))
	if err != nil {
		return "", err
	}
	for _, name := range []string{"cpu", "memory", "pids"} {
		if !strings.Contains(" "+strings.TrimSpace(string(b))+" ", " "+name+" ") {
			return "", fmt.Errorf("%s controller was not delegated", name)
		}
	}
	return root, nil
}

func cgroupWrite(dir, name, value string) error {
	// Open existing kernel files only. A missing controller is a failure,
	// never a regular file that happens to hold the desired number.
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	_, err = f.WriteString(value)
	return errors.Join(err, f.Close())
}

func newWebResources() (*webResources, error) {
	root, err := webResourceRoot()
	if err != nil {
		return nil, err
	}
	if err := cgroupWrite(root, "cgroup.subtree_control", "+cpu +memory +pids"); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(root, "web-")
	if err != nil {
		return nil, err
	}
	for _, limit := range webLimits {
		if err := cgroupWrite(dir, limit.file, limit.value); err != nil {
			os.Remove(dir)
			return nil, err
		}
	}
	fd, err := os.Open(dir)
	if err != nil {
		os.Remove(dir)
		return nil, err
	}
	return &webResources{dir: dir, fd: fd}, nil
}

// Reuse only an empty group. Leftover descendants cannot overlap the next
// renderer or evade accounting by surviving a wrapper exit.
func (r *webResources) empty() error {
	deadline := time.Now().Add(2 * time.Second)
	killed := false
	for {
		b, err := os.ReadFile(filepath.Join(r.dir, "cgroup.events"))
		if err != nil {
			return err
		}
		if strings.Contains(string(b), "populated 0\n") {
			return nil
		}
		// cgroup.kill is an event, not a persistent setting. Do not fire it at
		// an already empty group immediately before clone3 puts the new wrapper
		// there: that creates a kill-vs-start race on some kernels.
		if !killed {
			if err := cgroupWrite(r.dir, "cgroup.kill", "1"); err != nil {
				return err
			}
			killed = true
		}
		if time.Now().After(deadline) {
			return errors.New("web descendants did not leave their cgroup")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (r *webResources) close() error {
	return errors.Join(r.empty(), r.fd.Close(), os.Remove(r.dir))
}
