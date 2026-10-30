package main

import "syscall"

// The kernel stops a child whose supervisor dies (PR_SET_PDEATHSIG). The
// forking thread is locked for the child's whole life, which is what makes
// this reliable in a Go process.
func childAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}

// The kernel's death signal does it; the bus has nothing to watch.
func watchParent() {}

// Clearing the ambient set costs the supervisor nothing — its own permitted
// set is untouched, so it can still chown a socket on reload — and leaves a
// child with no capability at all.
const (
	prCapAmbient         = 47
	prCapAmbientClearAll = 4
)

func clearAmbient() error {
	if _, _, e := syscall.Syscall6(syscall.SYS_PRCTL, prCapAmbient, prCapAmbientClearAll, 0, 0, 0, 0); e != 0 {
		return e
	}
	return nil
}
