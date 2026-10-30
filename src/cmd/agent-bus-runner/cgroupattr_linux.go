package main

import (
	"syscall"

	"github.com/parf/ai-agent-bus/internal/cgroup"
)

// useCgroup starts the process straight in the agent's cgroup: none of it ever
// runs outside its limits, not even for the moment a move would take.
func useCgroup(attr *syscall.SysProcAttr, g *cgroup.Group) {
	attr.UseCgroupFD, attr.CgroupFD = true, g.FD()
}
