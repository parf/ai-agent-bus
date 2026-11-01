//go:build linux

package main

import "golang.org/x/sys/unix"

// renameNoReplace moves tmp under stage to name under parent, failing with
// EEXIST rather than replacing what is there: one atomic step.
func renameNoReplace(stage int, tmp string, parent int, name string) error {
	return unix.Renameat2(stage, tmp, parent, name, unix.RENAME_NOREPLACE)
}
