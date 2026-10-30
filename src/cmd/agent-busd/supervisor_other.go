//go:build !linux

package main

import (
	"os"
	"syscall"
	"time"
)

// No kernel death signal outside Linux: the bus child watches for its
// supervisor instead (watchParent).
func childAttr() *syscall.SysProcAttr { return nil }

// watchParent stops the bus, as a SIGTERM would, once its supervisor is gone
// and it has been handed to another parent. Under launchd the job's process
// group is stopped with it as well; this covers a supervisor killed outright.
func watchParent() {
	parent := os.Getppid()
	go func() {
		// An orphan is handed to launchd or a subreaper, never back.
		for os.Getppid() == parent {
			time.Sleep(time.Second)
		}
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()
}

// No ambient capabilities to clear outside Linux.
func clearAmbient() error { return nil }
