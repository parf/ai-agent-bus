package main

import (
	"syscall"

	"github.com/parf/ai-agent-bus/internal/cgroup"
)

// useCgroup does nothing here: macOS has no cgroups, so the runner never holds
// a group and an agent runs under the account's own limits.
func useCgroup(*syscall.SysProcAttr, *cgroup.Group) {}
