//go:build !linux

package main

import "golang.org/x/sys/unix"

// Only the Linux setup makes a person's files folder (a user-account runner is
// systemd's); the macOS setup has none, so nothing calls this there, and it
// refuses rather than replace.
func renameNoReplace(stage int, tmp string, parent int, name string) error { return unix.ENOTSUP }
